# Verifier fail: `ssotdeps verify` requires missing sibling backend/frontend artifacts

Attempt to run the required verifier command stack after adding a new malformed WorkItem candidate failed at the first command:

- `./bin/ssotdeps verify`

Observed output:

```
FAIL: consumption_links[4] consumer_path in sibling agent-cluster-backend: path "internal/contracts/work_item.go" does not exist (resolved to /Users/aske/github/agent_cluster_project_2/backend/internal/contracts/work_item.go)
FAIL: consumption_links[5] consumer_path in sibling agent-cluster-backend: path "internal/contracts/work_item_created.go" does not exist (resolved to /Users/aske/github/agent_cluster_project_2/backend/internal/contracts/work_item_created.go)
FAIL: consumption_links[6] consumer_path in sibling agent-cluster-frontend: path "lib/contracts_client/work_item.dart" does not exist (resolved to /Users/aske/github/agent_cluster_project_2/frontend/lib/contracts_client/work_item.dart)
FAIL: consumption_links[7] consumer_path in sibling agent-cluster-frontend: path "lib/contracts_client/work_item_created.dart" does not exist (resolved to /Users/aske/github/agent_cluster_project_2/frontend/lib/contracts_client/work_item_created.dart)
```

`./bin/conceptmap verify`, `./bin/decision validate`, `./bin/decision list --status accepted`, and `./bin/secretscan .` pass in this workspace.

No further candidate validation could be completed while this environment baseline failure is present.
