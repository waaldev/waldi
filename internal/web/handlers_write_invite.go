package web

import "net/http"

func (s *Server) handleOldWriteInvite(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/write", http.StatusMovedPermanently)
}
