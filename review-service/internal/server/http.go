package server

import (
	stdhttp "net/http"

	v1 "review-service/api/review/v1"
	"review-service/internal/conf"
	"review-service/internal/service"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	"github.com/go-kratos/kratos/v2/transport/http"
)

// NewHTTPServer new an HTTP server.
func NewHTTPServer(c *conf.Server, reviewer *service.ReviewService, logger log.Logger) *http.Server {
	var opts = []http.ServerOption{
		http.Filter(corsFilter()),
		http.Middleware(
			recovery.Recovery(),
			validate.Validator(),
		),
	}
	if c.Http.Network != "" {
		opts = append(opts, http.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, http.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, http.Timeout(c.Http.Timeout.AsDuration()))
	}
	srv := http.NewServer(opts...)
	registerUploadRoutes(srv, logger)
	registerRankRoutes(srv, reviewer, logger)
	registerPublicReviewRoutes(srv, reviewer, logger)
	v1.RegisterReviewHTTPServer(srv, reviewer)
	return srv
}

func corsFilter() http.FilterFunc {
	allowedOrigins := map[string]struct{}{
		"http://localhost:5173":     {},
		"http://127.0.0.1:5173":     {},
		"http://192.168.159.1:5173": {},
	}

	return func(next stdhttp.Handler) stdhttp.Handler {
		return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			origin := r.Header.Get("Origin")
			if _, ok := allowedOrigins[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			}

			if r.Method == stdhttp.MethodOptions {
				w.WriteHeader(stdhttp.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
