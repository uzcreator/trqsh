package api

import (
	"crypto/subtle"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/trqsh-uz/trqsh/internal/entitlerpc"
	"github.com/trqsh-uz/trqsh/pkg/authz"
)

// mountInternal registers the token-guarded entitlements RPC used by the edge.
func (s *Server) mountInternal(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(s.internalTokenGuard)
		r.Post(entitlerpc.PathAuthenticate, s.rpcAuthenticate)
		r.Post(entitlerpc.PathCheckBind, s.rpcCheckBind)
		r.Post(entitlerpc.PathReportUsage, s.rpcReportUsage)
	})
}

func (s *Server) internalTokenGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get(entitlerpc.HeaderToken)
		if s.cfg.InternalToken == "" || subtle.ConstantTimeCompare([]byte(got), []byte(s.cfg.InternalToken)) != 1 {
			writeError(w, http.StatusUnauthorized, "invalid internal token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) rpcAuthenticate(w http.ResponseWriter, r *http.Request) {
	var req entitlerpc.AuthRequest
	if !decode(w, r, &req) {
		return
	}
	accountID, plan, err := s.ent.Authenticate(r.Context(), req.APIKey)
	if err != nil {
		writeJSON(w, http.StatusOK, entitlerpc.AuthResponse{Error: "invalid api key"})
		return
	}
	writeJSON(w, http.StatusOK, entitlerpc.AuthResponse{AccountID: accountID, Plan: plan})
}

func (s *Server) rpcCheckBind(w http.ResponseWriter, r *http.Request) {
	var req entitlerpc.CheckBindRequest
	if !decode(w, r, &req) {
		return
	}
	dec, err := s.ent.CheckBind(r.Context(), authz.BindRequest{
		APIKey: req.APIKey, Type: req.Type, Subdomain: req.Subdomain,
		CustomHost: req.CustomHost, RemotePort: req.RemotePort, Region: req.Region,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entitlerpc.CheckBindResponse{
		Allow:             dec.Allow,
		AccountID:         dec.AccountID,
		Plan:              dec.Plan,
		Limits:            dec.Limits,
		AssignedSubdomain: dec.AssignedSubdomain,
		ErrorCode:         dec.ErrorCode,
		ErrorMessage:      dec.ErrorMessage,
	})
}

func (s *Server) rpcReportUsage(w http.ResponseWriter, r *http.Request) {
	var req entitlerpc.UsageRequest
	if !decode(w, r, &req) {
		return
	}
	_ = s.ent.ReportUsage(r.Context(), authz.Usage{
		AccountID: req.AccountID, TunnelID: req.TunnelID, BytesIn: req.BytesIn, BytesOut: req.BytesOut,
		Requests: req.Requests, WindowStart: req.WindowStart, WindowEnd: req.WindowEnd,
	})
	w.WriteHeader(http.StatusOK)
}
