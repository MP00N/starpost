package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// GeeTest v4 verification.
//
// v4 does not hand out a single opaque token the way Turnstile and hCaptcha
// do. The widget returns four fields, and the server must derive a signature
// from one of them with the private key before asking GeeTest's own validator:
//
//	sign_token = hex(HMAC-SHA256(key=captcha_key, message=lot_number))
//
// Skipping the signature and posting the four fields straight through is the
// usual mistake — the endpoint answers "fail" and it looks like a widget
// problem rather than a missing HMAC.

// geetestPayload is what the widget hands to the browser.
type geetestPayload struct {
	LotNumber     string `json:"lot_number"`
	CaptchaOutput string `json:"captcha_output"`
	PassToken     string `json:"pass_token"`
	GenTime       string `json:"gen_time"`
}

type geetestCheckResponse struct {
	Status string `json:"status"`
	Result string `json:"result"`
	Reason string `json:"reason"`
}

var geetestVerifyURL = "https://gcaptcha4.geetest.com/validate"
var geetestHTTPClient = &http.Client{Timeout: 10 * time.Second}

func GeetestCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Same transport rule as hCaptcha: the four GeeTest fields travel as
		// one JSON string, by default in the shared `turnstile` parameter.
		raw := c.Query("geetest")
		if raw == "" {
			raw = c.Query("turnstile")
		}
		if raw == "" {
			common.ApiErrorT(c, "GeeTest token is empty")
			c.Abort()
			return
		}
		if strings.TrimSpace(common.GeetestCaptchaKey) == "" || strings.TrimSpace(common.GeetestCaptchaId) == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "GeeTest configuration is incomplete",
			})
			c.Abort()
			return
		}

		var payload geetestPayload
		if err := common.UnmarshalJsonStr(raw, &payload); err != nil {
			common.ApiErrorT(c, "GeeTest token is malformed")
			c.Abort()
			return
		}
		if payload.LotNumber == "" || payload.CaptchaOutput == "" || payload.PassToken == "" || payload.GenTime == "" {
			common.ApiErrorT(c, "GeeTest token is incomplete")
			c.Abort()
			return
		}

		// gen_time is when the challenge was solved. Reject anything older than
		// two minutes so a captured payload cannot be replayed later.
		if genTime, err := strconv.ParseInt(payload.GenTime, 10, 64); err == nil {
			if time.Now().UnixMilli()-genTime > 120_000 {
				common.ApiErrorT(c, "Verification expired, please refresh and retry")
				c.Abort()
				return
			}
		}

		form := url.Values{
			"lot_number":     {payload.LotNumber},
			"captcha_output": {payload.CaptchaOutput},
			"pass_token":     {payload.PassToken},
			"gen_time":       {payload.GenTime},
			"sign_token":     {geetestSignToken(common.GeetestCaptchaKey, payload.LotNumber)},
		}
		endpoint := geetestVerifyURL + "?captcha_id=" + url.QueryEscape(common.GeetestCaptchaId)
		req, err := http.NewRequestWithContext(
			c.Request.Context(),
			http.MethodPost,
			endpoint,
			strings.NewReader(form.Encode()),
		)
		if err != nil {
			common.ApiError(c, err)
			c.Abort()
			return
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := geetestHTTPClient.Do(req)
		if err != nil {
			common.SysLog(err.Error())
			common.ApiErrorT(c, "GeeTest verification is unavailable, try again")
			c.Abort()
			return
		}
		defer resp.Body.Close()

		var result geetestCheckResponse
		if err := common.DecodeJson(resp.Body, &result); err != nil {
			common.SysLog(err.Error())
			common.ApiErrorT(c, "GeeTest verification is unavailable, try again")
			c.Abort()
			return
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices ||
			result.Status != "success" || result.Result != "success" {
			if result.Reason != "" {
				common.SysLog("geetest rejected: " + result.Reason)
			}
			common.ApiErrorT(c, "Verification failed, please refresh and retry")
			c.Abort()
			return
		}

		c.Next()
	}
}

func geetestSignToken(captchaKey string, lotNumber string) string {
	mac := hmac.New(sha256.New, []byte(captchaKey))
	mac.Write([]byte(lotNumber))
	return hex.EncodeToString(mac.Sum(nil))
}
