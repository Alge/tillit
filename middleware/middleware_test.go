package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Alge/tillit/models"
	"github.com/Alge/tillit/requestdata"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestIsAdmin_NoUser(t *testing.T) {
	handler := IsAdmin(okHandler(), nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestIsAdmin_NonAdminUser(t *testing.T) {
	handler := IsAdmin(okHandler(), nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = requestdata.WithUser(req, &models.User{IsAdmin: false})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestIsAdmin_AdminUser(t *testing.T) {
	handler := IsAdmin(okHandler(), nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = requestdata.WithUser(req, &models.User{IsAdmin: true})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}
