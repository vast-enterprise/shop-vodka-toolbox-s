// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package handler

import (
	"net/http"

	"toolbox/internal/logic"
	"toolbox/internal/svc"
	"toolbox/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func ModelCompressHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ModelCompressRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewModelCompressHandlerLogic(r.Context(), svcCtx)
		resp, err := l.Handle(&req, svcCtx)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
