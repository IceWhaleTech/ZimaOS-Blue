# Parallel search provider fixture

This dataset accompanies `TestParallelSearchHarnessWorkflow` in
`server/internal/harness/drivers/parallel_search_test.go`. The test creates the
named local workflow and a deterministic MCP server, imports the manifest into
Blue Harness, and runs the existing WorkflowDriver, workflow skill action,
ToolGateway and web-query search dispatcher. Harness scores workflow completion;
the test also checks the saved workflow node output for the source URL, provider
and excerpt. No model or external credentials are needed.

Run from `server/`:

```sh
go test -tags fts5 ./internal/harness/drivers -run '^TestParallelSearchHarnessWorkflow$' -v
```

The named fixture workflow is created by the test. Importing this dataset into an
unrelated Blue installation does not create that workflow. The provider's native
tests cover unchanged defaults, overrides, disabled providers, permissions,
protocol errors, malformed results, quotas, cancellation and timeouts.
