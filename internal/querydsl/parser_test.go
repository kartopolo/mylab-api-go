package querydsl

import (
	"context"
	"strings"
	"testing"

	"mylab-api-go/internal/database/eloquent"
)

func TestParseAndBuildSQL_TenantInjected(t *testing.T) {
	reg := NewRegistry()
	reg.Register("menu", func() eloquent.Schema {
		return eloquent.Schema{
			Table:      "menu",
			PrimaryKey: "id",
			Columns:    []string{"id", "menu_name", "app_name", "company_id"},
			Casts:      map[string]eloquent.CastType{"id": eloquent.CastInt, "company_id": eloquent.CastInt},
		}
	})

	spec, err := ParseLaravelQuery("table('menu as m')->select('m.id','m.menu_name')->where('m.id','=','1')->orderby('m.id','desc')->take(1)")
	if err != nil {
		t.Fatalf("ParseLaravelQuery err: %v", err)
	}
	built, err := BuildSQL(context.TODO(), reg, 7, spec)
	if err != nil {
		t.Fatalf("BuildSQL err: %v", err)
	}
	if built.SQL == "" {
		t.Fatalf("empty SQL")
	}
	// Must include tenant filter
	if !strings.Contains(built.SQL, "m.company_id") {
		t.Fatalf("expected tenant filter in SQL, got: %s", built.SQL)
	}
	if len(built.Args) == 0 {
		t.Fatalf("expected args")
	}
}
