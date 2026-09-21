package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tuzi/cdk-recharge-system/internal/db"
)

func resolveCDKInput(c *gin.Context, input string) (string, bool) {
	code, err := db.ResolveCDKCode(input)
	if err == nil {
		return code, true
	}
	if errors.Is(err, db.ErrCDKAlias) {
		c.JSON(http.StatusBadRequest, gin.H{"error": db.ErrCDKAlias.Error(), "error_code": "CDK_INVALID"})
	} else {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "暂无法核对卡密，请稍后重试"})
	}
	return "", false
}
