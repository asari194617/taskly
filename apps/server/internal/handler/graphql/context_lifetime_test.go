package gqlH

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	graphql "github.com/graphql-go/graphql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type requestMarkerKey struct{}

func TestGraphQLRootAndNestedResolversKeepRequestCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), requestMarkerKey{}, "original"), time.Second)
	defer cancel()
	deadline, _ := ctx.Deadline()
	var calls atomic.Int32
	checkContext := func(params graphql.ResolveParams) {
		t.Helper()
		actual, ok := params.Context.Deadline()
		if !ok || !actual.Equal(deadline) || params.Context.Done() != ctx.Done() || params.Context.Value(requestMarkerKey{}) != "original" {
			t.Error("resolver lost the original cancellable request context")
		}
		calls.Add(1)
	}
	nested := graphql.NewObject(graphql.ObjectConfig{Name: "OwnedNested", Fields: graphql.Fields{
		"value": {Type: graphql.String, Resolve: func(params graphql.ResolveParams) (any, error) {
			checkContext(params)
			return "value", nil
		}},
	}})
	root := graphql.NewObject(graphql.ObjectConfig{Name: "OwnedRoot", Fields: graphql.Fields{
		"nested": {Type: nested, Resolve: func(params graphql.ResolveParams) (any, error) {
			checkContext(params)
			return map[string]any{}, nil
		}},
	}})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: root})
	if err != nil {
		t.Fatal(err)
	}
	introspection := schema.TypeMap()["__Type"].(*graphql.Object).Fields()["name"]
	introspectionResolver := reflect.ValueOf(introspection.Resolve).Pointer()
	bindRequestResolvers(&schema)
	if reflect.ValueOf(introspection.Resolve).Pointer() != introspectionResolver {
		t.Fatal("binding request resolvers changed a library-global introspection field")
	}
	execution := graphQLExecutionContext(ctx)
	if execution.Done() != nil {
		t.Fatal("executor could still return before its resolver goroutine finishes")
	}
	result := graphql.Do(graphql.Params{Schema: schema, RequestString: `{ nested { value } }`, Context: execution})
	if len(result.Errors) != 0 || calls.Load() != 2 {
		t.Fatalf("nested execution failed: %+v", result.Errors)
	}
	result = graphql.Do(graphql.Params{Schema: schema, RequestString: `{ __type(name:"OwnedNested") { name } }`, Context: execution})
	if len(result.Errors) != 0 {
		t.Fatal("introspection behavior changed")
	}
}

func TestGraphQLStopsLaterMutationAliasesAfterRequestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	firstFinished := false
	secondCalls := 0
	query := graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: graphql.Fields{"noop": {Type: graphql.Boolean}}})
	mutation := graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: graphql.Fields{
		"first": {Type: graphql.Boolean, Resolve: func(params graphql.ResolveParams) (any, error) {
			if params.Context != ctx {
				t.Error("mutation received a detached I/O context")
			}
			cancel()
			firstFinished = true
			return true, nil
		}},
		"second": {Type: graphql.Boolean, Resolve: func(graphql.ResolveParams) (any, error) { secondCalls++; return true, nil }},
	}})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: query, Mutation: mutation})
	if err != nil {
		t.Fatal(err)
	}
	bindRequestResolvers(&schema)
	result := graphql.Do(graphql.Params{Schema: schema, RequestString: `mutation { first second }`, Context: graphQLExecutionContext(ctx)})
	if !firstFinished || secondCalls != 0 || len(result.Errors) == 0 {
		t.Fatalf("cancelled execution did not finish its active resolver and stop the next one: finished=%v calls=%d errors=%v", firstFinished, secondCalls, result.Errors)
	}
}

func TestGraphQLSharedSchemaDoesNotCaptureAnotherRequestsContext(t *testing.T) {
	query := graphql.NewObject(graphql.ObjectConfig{Name: "ConcurrentQuery", Fields: graphql.Fields{
		"who": {Type: graphql.String, Resolve: func(params graphql.ResolveParams) (any, error) { return params.Context.Value(requestMarkerKey{}), nil }},
	}})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: query})
	if err != nil {
		t.Fatal(err)
	}
	bindRequestResolvers(&schema)
	var pending sync.WaitGroup
	for index := range 24 {
		pending.Add(1)
		go func() {
			defer pending.Done()
			marker := fmt.Sprint(index)
			ctx := context.WithValue(context.Background(), requestMarkerKey{}, marker)
			result := graphql.Do(graphql.Params{Schema: schema, RequestString: `{ who }`, Context: graphQLExecutionContext(ctx)})
			if len(result.Errors) != 0 || result.Data.(map[string]any)["who"] != marker {
				t.Error("shared schema captured another request's context")
			}
		}()
	}
	pending.Wait()
}

func TestRequestBoundResolverAcceptsGraphQLDefaultContext(t *testing.T) {
	value, err := requestBoundResolver(func(params graphql.ResolveParams) (any, error) { return params.Context != nil, nil })(graphql.ResolveParams{})
	if err != nil || value != true {
		t.Fatal("default GraphQL context no longer works")
	}
}

func TestGraphQLNestedSubjectQueryUsesRequestCancellation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.Subject{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&domain.Subject{ID: 1, NameJa: "test"}).Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var queryContext context.Context
	if err := db.Callback().Query().Before("gorm:query").Register("test:nested-cancel", func(tx *gorm.DB) {
		queryContext = tx.Statement.Context
		cancel()
		select {
		case <-queryContext.Done():
		case <-time.After(100 * time.Millisecond):
			tx.AddError(fmt.Errorf("nested subject query ignored request cancellation"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	types := NewHandler(db, nil, nil).createGQLTypes()
	query := graphql.NewObject(graphql.ObjectConfig{Name: "NestedDBQuery", Fields: graphql.Fields{
		"subject": {Type: types.SubjectType, Resolve: func(graphql.ResolveParams) (any, error) {
			return map[string]any{"id": 1}, nil
		}},
	}})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: query})
	if err != nil {
		t.Fatal(err)
	}
	bindRequestResolvers(&schema)
	graphql.Do(graphql.Params{Schema: schema, RequestString: `{ subject { name } }`, Context: graphQLExecutionContext(ctx)})
	if queryContext == nil || queryContext.Err() != context.Canceled {
		t.Fatal("nested subject DB query did not receive request cancellation")
	}
}
