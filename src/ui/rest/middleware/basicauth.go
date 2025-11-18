package middleware

import (
	"context"
	"github.com/gofiber/fiber/v2"
)

type AuthorizationValue string
type CompanyNidValue string

func BasicAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := string(c.Request().Header.Peek("Authorization"))
		if token != "" {
			ctx := context.WithValue(c.Context(), AuthorizationValue("BASIC_AUTH"), token)
			c.SetUserContext(ctx)
		}

		return c.Next()
	}
}

func GetCompanyNidFromContext(ctx context.Context) string {
	if val := ctx.Value(CompanyNidValue("COMPANY_NID")); val != nil {
		if companyNid, ok := val.(string); ok {
			return companyNid
		}
	}
	return ""
}

func SetCompanyNidInContext(ctx context.Context, companyNid string) context.Context {
	return context.WithValue(ctx, CompanyNidValue("COMPANY_NID"), companyNid)
}
