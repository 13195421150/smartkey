package server

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRetiredExchangeRoutesAreClosedAndRotationRequiresAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	setupRoutes(router)
	for _, path := range []string{"/partner/swap", "/partner/swap/test", "/a/swap", "/a/swap/test", "/api/v1/public/cdk/exchange"} {
		for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(method, path, nil))
			if w.Code != 410 {
				t.Fatalf("%s %s = %d", method, path, w.Code)
			}
		}
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/admin/cardplatform/cdks/7/rotate", nil))
	if w.Code != 401 {
		t.Fatalf("rotation without auth = %d", w.Code)
	}
}
