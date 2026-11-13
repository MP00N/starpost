package middleware

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// CaptchaCheck dispatches to the configured human-verification provider.
//
// One entry point rather than three booleans: with independent switches an
// operator can turn two providers on at once, and then each endpoint has to
// guess which token it should have received. `CaptchaType` makes that state
// unrepresentable.
//
// An unknown value falls back to Turnstile, which is the historical default —
// an install that upgrades with no configuration keeps working.
func CaptchaCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch common.CaptchaType {
		case "none":
			c.Next()
		case "hcaptcha":
			HCaptchaCheck()(c)
		case "geetest":
			GeetestCheck()(c)
		case "turnstile", "":
			TurnstileCheck()(c)
		default:
			TurnstileCheck()(c)
		}
	}
}
