package middleware

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// hCaptcha verification, ported from the fork at Steve3184/new-api so both
// codebases speak the same protocol and the same option names.
//
// The token arrives in query params rather than a JSON body because this runs
// before body binding, exactly like the Turnstile middleware it sits beside.
type hCaptchaCheckResponse struct {
	Success bool `json:"success"`
}

var hCaptchaVerifyURL = "https://api.hcaptcha.com/siteverify"
var hCaptchaHTTPClient = &http.Client{Timeout: 10 * time.Second}

func HCaptchaCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Transport note: every provider rides the existing `turnstile` query
		// parameter so the frontend API layer and the auth forms keep one
		// contract regardless of which widget is active. Provider-specific
		// names are accepted too, for direct API callers.
		token := c.Query("hcaptcha")
		if token == "" {
			token = c.Query("h-captcha-response")
		}
		if token == "" {
			token = c.Query("turnstile")
		}
		if token == "" {
			common.ApiErrorT(c, "hCaptcha token is empty")
			c.Abort()
			return
		}
		// Fail closed on a half-configured install: verifying against an empty
		// secret would return success:false anyway, but the message would blame
		// the user instead of the configuration.
		if strings.TrimSpace(common.HCaptchaSecretKey) == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "hCaptcha configuration is incomplete",
			})
			c.Abort()
			return
		}

		form := url.Values{
			"secret":   {common.HCaptchaSecretKey},
			"response": {token},
			"remoteip": {c.ClientIP()},
			"sitekey":  {common.HCaptchaSiteKey},
		}
		req, err := http.NewRequestWithContext(
			c.Request.Context(),
			http.MethodPost,
			hCaptchaVerifyURL,
			strings.NewReader(form.Encode()),
		)
		if err != nil {
			common.ApiError(c, err)
			c.Abort()
			return
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := hCaptchaHTTPClient.Do(req)
		if err != nil {
			common.SysLog(err.Error())
			common.ApiErrorT(c, "hCaptcha verification is unavailable, try again")
			c.Abort()
			return
		}
		defer resp.Body.Close()

		var result hCaptchaCheckResponse
		if err := common.DecodeJson(resp.Body, &result); err != nil {
			common.SysLog(err.Error())
			common.ApiErrorT(c, "hCaptcha verification is unavailable, try again")
			c.Abort()
			return
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || !result.Success {
			// One message for expired, forged and replayed tokens alike: a
			// precise reason would tell an attacker which part to fix.
			common.ApiErrorT(c, "Verification failed, please refresh and retry")
			c.Abort()
			return
		}

		c.Next()
	}
}
