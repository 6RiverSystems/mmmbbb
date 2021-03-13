package middleware

import (
	"database/sql"

	"github.com/gin-gonic/gin"

	"go.6river.tech/gosix/ginmiddleware"
	"go.6river.tech/mmmbbb/ent"
)

// thin shims around the generic ent middleware to add app-specific type safety.
// these rely on a custom method added to the Client.

func Client(c *gin.Context, name string) *ent.Client {
	return ginmiddleware.Client(c, name).(*ent.Client)
}

func WithTransaction(
	name string,
	opts *sql.TxOptions,
	controls ...ginmiddleware.TransactionControl,
) gin.HandlerFunc {
	return ginmiddleware.WithTransaction(name, opts, controls...)
}

func Transaction(c *gin.Context, name string) *ent.Tx {
	return ginmiddleware.Transaction(c, name).(*ent.Tx)
}
