package action

import (
	"context"

	"github.com/erniealice/pyeza-golang/view"

	"github.com/erniealice/hybra-golang/views/omni_search/model"
)

// NewDialogAction returns the GET /action/omni-search/dialog handler. It renders
// the palette SHELL only — the lyngua-fied dialog chrome (input wired to the
// results endpoint, loading indicator, footer hints). It calls no use case and
// carries no tenant data, so it is pure chrome; the Session middleware already
// rejects unauthenticated requests before the handler body runs.
func NewDialogAction(deps *Deps) view.View {
	return view.ViewFunc(func(_ context.Context, _ *view.ViewContext) view.ViewResult {
		minChars := deps.minChars()
		data := &model.DialogData{
			Labels:   deps.Labels,
			Routes:   deps.Routes,
			MinChars: minChars,
			Hint:     resolveHint(deps.Labels.Hint, minChars),
		}
		res := view.OK("omni-search-dialog", data)
		res.Headers = noStoreHeaders()
		return res
	})
}
