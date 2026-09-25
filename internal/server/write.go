package server

import (
	"context"
	"errors"
)

func (a *app) write(ctx context.Context, method, path string, p arguments, body map[string]any, receipt bool) (any, error) {
	key := stringArg(p, "request_id")
	if key != "" {
		body["requestId"] = key
	}
	if !receipt {
		key = ""
	}
	return a.request(ctx, method, a.path(path), body, key)
}
func copyArgs(p arguments, fields map[string]string) map[string]any {
	body := map[string]any{}
	for source, target := range fields {
		if v, ok := p[source]; ok {
			body[target] = v
		}
	}
	return body
}
func confirmProps() map[string]any {
	return map[string]any{"confirm_name": text("Exact current service/project/domain name. The host must separately obtain user approval.", 255), "request_id": requestKey()}
}
func (a *app) confirmService(ctx context.Context, p arguments) error {
	data, err := a.get(ctx, servicePath(p))
	if err != nil {
		return err
	}
	body, _ := data.(map[string]any)
	service, _ := body["service"].(map[string]any)
	if stringArg(p, "confirm_name") == "" || stringArg(service, "name") != stringArg(p, "confirm_name") {
		return errors.New("confirm_name must match the selected service's current name")
	}
	return nil
}
func (a *app) writeTools() {
	projectProps := map[string]any{"name": text("Project name", 80), "description": text("Project description", 500), "color": enum("Project color", "violet", "blue", "green", "orange", "pink", "gray"), "request_id": requestKey()}
	a.add("create_project", "Create a project and its default Production environment. Requires write mode and backend permission.", object(projectProps, "name", "request_id"), true, false, true, func(ctx context.Context, p arguments) (any, error) {
		return a.write(ctx, "POST", "projects", p, copyArgs(p, map[string]string{"name": "name", "description": "description", "color": "color"}), true)
	})
	a.add("update_project", "Update project display settings.", object(merge(projectProps, map[string]any{"project_id": identifier("Project UUID")}), "project_id", "request_id"), true, true, true, func(ctx context.Context, p arguments) (any, error) {
		return a.write(ctx, "PATCH", "projects/"+stringArg(p, "project_id"), p, copyArgs(p, map[string]string{"name": "name", "description": "description", "color": "color"}), true)
	})
	a.add("delete_project", "Delete an empty project after explicit user approval and exact-name confirmation. Backend requires admin access.", object(merge(confirmProps(), map[string]any{"project_id": identifier("Project UUID")}), "project_id", "confirm_name", "request_id"), true, true, true, func(ctx context.Context, p arguments) (any, error) {
		return a.write(ctx, "DELETE", "projects/"+stringArg(p, "project_id"), p, map[string]any{"confirm": p["confirm_name"]}, true)
	})
	configuration := map[string]any{"type": "object", "description": "Service configuration from the catalog/API: repository, branch, runtime, plan, build/start command, region and related options. Backend validates fields, entitlements and paid-compute consent.", "additionalProperties": true}
	serviceFields := map[string]string{"name": "name", "kind": "kind", "configuration": "configuration", "project_id": "projectId", "environment_id": "environmentId"}
	servicePropsCreate := map[string]any{"name": text("Lowercase service name using letters, numbers and hyphens", 63), "kind": enum("Service type", "web", "static", "private", "worker", "cron", "postgres", "redis"), "configuration": configuration, "project_id": identifier("Optional project UUID; also provide environment_id"), "environment_id": identifier("Environment UUID belonging to project_id"), "request_id": requestKey()}
	a.add("create_service", "Save a new service configuration. This does not deploy it or authorize charges. Use get_catalog and deploy_service separately.", object(servicePropsCreate, "name", "kind", "request_id"), true, false, true, func(ctx context.Context, p arguments) (any, error) {
		return a.write(ctx, "POST", "services", p, copyArgs(p, serviceFields), true)
	})
	updateProps := merge(servicePropsCreate, serviceProps())
	delete(updateProps, "kind")
	for _, field := range []string{"project_id", "environment_id"} {
		updateProps[field] = map[string]any{"type": []string{"string", "null"}, "pattern": identifier("")["pattern"], "description": "Assignment UUID, or null to clear it; project and environment must stay consistent"}
	}
	a.add("update_service", "Update a service's configuration. Routing changes can affect live traffic; build/start changes may require a separate deployment. Backend enforces plan restrictions.", object(updateProps, "service_id", "request_id"), true, true, true, func(ctx context.Context, p arguments) (any, error) {
		return a.write(ctx, "PATCH", servicePath(p), p, copyArgs(p, serviceFields), true)
	})
	a.add("delete_service", "Delete service configuration and queue infrastructure removal if deployed. Destructive: require explicit user approval and exact-name confirmation.", object(merge(serviceProps(), confirmProps()), "service_id", "confirm_name", "request_id"), true, true, true, func(ctx context.Context, p arguments) (any, error) {
		return a.write(ctx, "DELETE", servicePath(p), p, map[string]any{"confirm": p["confirm_name"]}, true)
	})
	a.add("deploy_service", "Queue a deployment from the configured repository/image. Return value acknowledges queueing, not readiness. Reuse request_id for an identical retry. Paid instances must already be accepted in Billing.", object(merge(serviceProps(), map[string]any{"request_id": requestKey(), "commit_sha": text("Optional full commit SHA", 40), "clear_cache": map[string]any{"type": "boolean", "description": "Clear build cache before deployment; cannot combine with commit_sha"}}), "service_id", "request_id"), true, true, true, func(ctx context.Context, p arguments) (any, error) {
		body := copyArgs(p, map[string]string{"commit_sha": "commitSha"})
		target := servicePath(p) + "/deploys"
		if boolArg(p, "clear_cache") {
			if stringArg(p, "commit_sha") != "" {
				return nil, errors.New("clear_cache cannot be combined with commit_sha")
			}
			target = servicePath(p) + "/actions"
			body["action"] = "clear-cache"
		}
		return a.write(ctx, "POST", target, p, body, true)
	})
	a.add("service_action", "Restart, suspend or resume an existing service after explicit approval. Inspect get_operation/get_service until the operation actually completes.", object(merge(merge(serviceProps(), confirmProps()), map[string]any{"action": enum("Runtime operation", "restart", "suspend", "resume")}), "service_id", "action", "confirm_name", "request_id"), true, true, true, func(ctx context.Context, p arguments) (any, error) {
		if err := a.confirmService(ctx, p); err != nil {
			return nil, err
		}
		return a.write(ctx, "POST", servicePath(p)+"/actions", p, map[string]any{"action": p["action"]}, true)
	})
	for _, action := range []string{"cancel", "rollback"} {
		a.add(action+"_deployment", action+" a deployment after user approval. A rollback queues a new deployment; cancellation is asynchronous.", object(merge(merge(serviceProps(), confirmProps()), map[string]any{"deployment_id": identifier("Deployment UUID")}), "service_id", "deployment_id", "confirm_name", "request_id"), true, true, true, func(ctx context.Context, p arguments) (any, error) {
			if err := a.confirmService(ctx, p); err != nil {
				return nil, err
			}
			return a.write(ctx, "POST", servicePath(p)+"/deploys/"+stringArg(p, "deployment_id")+"/"+action, p, map[string]any{"confirm": p["confirm_name"]}, true)
		})
	}
	a.add("create_environment", "Create a project environment within its plan limit. This endpoint has no replay receipts; inspect before retrying an uncertain response.", object(map[string]any{"project_id": identifier("Project UUID"), "name": text("Environment name", 80), "protected": map[string]any{"type": "boolean"}, "isolated": map[string]any{"type": "boolean"}}, "project_id", "name"), true, false, false, func(ctx context.Context, p arguments) (any, error) {
		return a.write(ctx, "POST", "environments", p, copyArgs(p, map[string]string{"project_id": "projectId", "name": "name", "protected": "protected", "isolated": "isolated"}), false)
	})
	a.add("add_domain", "Add a custom domain and queue DNS verification. Requires a domain the user owns; this does not purchase a domain or edit DNS. Inspect before retrying an uncertain response.", object(merge(serviceProps(), map[string]any{"name": text("Public hostname, without scheme or path", 253)}), "service_id", "name"), true, false, false, func(ctx context.Context, p arguments) (any, error) {
		return a.write(ctx, "POST", servicePath(p)+"/domains", p, map[string]any{"name": p["name"]}, false)
	})
	a.add("verify_domain", "Queue DNS and certificate verification for an existing domain. Check list_domains for the actual result.", object(merge(serviceProps(), map[string]any{"domain_id": identifier("Domain UUID")}), "service_id", "domain_id"), true, false, false, func(ctx context.Context, p arguments) (any, error) {
		return a.write(ctx, "POST", servicePath(p)+"/domains/"+stringArg(p, "domain_id")+"/verify", p, map[string]any{}, false)
	})
	a.add("remove_domain", "Queue removal of a domain and its live route after explicit user approval. Exact hostname confirmation is required.", object(merge(serviceProps(), map[string]any{"domain_id": identifier("Domain UUID"), "confirm_name": text("Exact hostname being removed", 253)}), "service_id", "domain_id", "confirm_name"), true, true, false, func(ctx context.Context, p arguments) (any, error) {
		return a.write(ctx, "DELETE", servicePath(p)+"/domains/"+stringArg(p, "domain_id"), p, map[string]any{"confirm": p["confirm_name"]}, false)
	})
	a.add("create_backup", "Queue a logical export of a running managed database. Inspect list_backups for completion; export/retention charges and plan rules apply.", object(merge(serviceProps(), map[string]any{"request_id": requestKey()}), "service_id", "request_id"), true, false, false, func(ctx context.Context, p arguments) (any, error) {
		return a.write(ctx, "POST", servicePath(p)+"/backups", p, map[string]any{}, false)
	})
	a.add("detect_repository", "Detect an accessible repository's framework and build settings. Does not deploy it. Requires a write-scoped key because the API uses POST.", object(map[string]any{"repository": text("owner/repository or GitHub repository URL", 300), "branch": text("Git branch (default main)", 255), "root_directory": text("Relative monorepo root", 255), "connection_id": identifier("Optional GitHub connection UUID")}, "repository"), true, false, false, func(ctx context.Context, p arguments) (any, error) {
		body := copyArgs(p, map[string]string{"repository": "repository", "branch": "branch", "root_directory": "rootDirectory", "connection_id": "connectionId"})
		if _, ok := body["branch"]; !ok {
			body["branch"] = "main"
		}
		return a.write(ctx, "POST", "github/detect", p, body, false)
	})
	if a.options.AllowSecrets {
		a.secretTools()
	}
	if a.options.AllowExec {
		a.executionTools()
	}
}

func (a *app) secretTools() {
	for _, spec := range []struct{ name, path string }{{"variable", "variables"}, {"secret_file", "secret-files"}} {
		props := merge(serviceProps(), map[string]any{"key": text("Variable key or relative secret-file name", 255), "value": text("Secret contents; these arguments may be stored by the MCP host", 65536), "secret_id": identifier("Existing secret UUID for update; omit to create"), "request_id": requestKey()})
		a.add("set_"+spec.name, "Create or update an encrypted "+spec.name+". Values pass through the MCP host; obtain explicit permission before providing secrets. The response contains metadata only.", object(props, "service_id", "key", "value", "request_id"), true, true, true, func(ctx context.Context, p arguments) (any, error) {
			method, target := "POST", servicePath(p)+"/"+spec.path
			if id := stringArg(p, "secret_id"); id != "" {
				method = "PATCH"
				target += "/" + id
			}
			return a.write(ctx, method, target, p, map[string]any{"key": p["key"], "value": p["value"]}, true)
		})
		a.add("delete_"+spec.name, "Delete a "+spec.name+" after explicit approval; this can break later builds or runtime configuration.", object(merge(serviceProps(), map[string]any{"secret_id": identifier("Secret UUID"), "request_id": requestKey()}), "service_id", "secret_id", "request_id"), true, true, true, func(ctx context.Context, p arguments) (any, error) {
			return a.write(ctx, "DELETE", servicePath(p)+"/"+spec.path+"/"+stringArg(p, "secret_id"), p, map[string]any{}, true)
		})
	}
}
func (a *app) executionTools() {
	a.add("run_job", "Execute a one-off command in a service workload or run its configured cron command. Commands can change or delete application data. Require explicit user approval; paid-instance and runtime checks apply. Read get_job until completion.", object(merge(merge(serviceProps(), confirmProps()), map[string]any{"command": text("Command to execute; omit for the configured cron command", 8192)}), "service_id", "confirm_name", "request_id"), true, true, false, func(ctx context.Context, p arguments) (any, error) {
		if err := a.confirmService(ctx, p); err != nil {
			return nil, err
		}
		return a.write(ctx, "POST", servicePath(p)+"/runs", p, copyArgs(p, map[string]string{"command": "command"}), false)
	})
	a.add("cancel_job", "Request cancellation of a running job after user approval; check its actual final state.", object(merge(serviceProps(), map[string]any{"job_id": identifier("Job UUID"), "confirm_name": text("Exact service name", 63)}), "service_id", "job_id", "confirm_name"), true, true, false, func(ctx context.Context, p arguments) (any, error) {
		if err := a.confirmService(ctx, p); err != nil {
			return nil, err
		}
		return a.write(ctx, "POST", servicePath(p)+"/runs/"+stringArg(p, "job_id")+"/cancel", p, map[string]any{}, false)
	})
}
