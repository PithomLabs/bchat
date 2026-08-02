> ## Documentation Index
> Fetch the complete documentation index at: https://www.cockroachlabs.com/llms.txt
> Use this file to discover all available pages before exploring further.

# Test Your Application Locally

export const InternalLink = ({version, path = "", children, ...props}) => {
  let detectedVersion = version || "stable";
  if (typeof window !== 'undefined' && !version) {
    const match = window.location.pathname.match(/\/docs\/([^/]+)/);
    if (match) {
      detectedVersion = match[1];
    }
  }
  const normalizedPath = path.startsWith("/") ? path.slice(1) : path;
  return <a href={`/docs/${detectedVersion}/${normalizedPath}`} {...props}>
      {children}
    </a>;
};

This page documents best practices for unit testing applications built on CockroachDB in a local environment.

If you are deploying a self-hosted cluster, see the <InternalLink path="recommended-production-settings">Production Checklist</InternalLink> for information about preparing your cluster for production.

<Danger>
  The settings described on this page are **not recommended** for use in production clusters. They are only recommended for use during unit testing and continuous integration testing (CI).
</Danger>

## Use a local, single-node cluster with in-memory storage

The <InternalLink path="cockroach-start-single-node">`cockroach start-single-node`</InternalLink> command below starts a single-node, insecure cluster with <InternalLink path="cockroach-start-single-node#store">in-memory storage</InternalLink>. Using in-memory storage improves the speed of the cluster for local testing purposes.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
cockroach start-single-node --insecure --store=type=mem,size=0.25 --advertise-addr=localhost
```

We recommend the following additional <InternalLink path="cluster-settings">cluster settings</InternalLink> and <InternalLink path="sql-statements#data-definition-statements">SQL statements</InternalLink> for improved performance during functional unit testing and continuous integration testing. In particular, some of these settings will increase the performance of <InternalLink path="online-schema-changes">schema changes</InternalLink>, since repeated <InternalLink path="create-schema">creation</InternalLink> and <InternalLink path="drop-schema">dropping</InternalLink> of schemas are common in automated testing.

| Setting                                                      | Value   | Description                                                                                                                                                                                                                                                                       |
| ------------------------------------------------------------ | ------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `kv.range_merge.queue_interval`                              | `50ms`  | Frequent <InternalLink path="create-table">`CREATE TABLE`</InternalLink> or <InternalLink path="drop-table">`DROP TABLE`</InternalLink> creates extra ranges, which we want to merge more quickly. In real usage, range merges are rate limited because they require rebalancing. |
| `jobs.registry.interval.gc`                                  | `30s`   | CockroachDB executes internal queries that scan the <InternalLink path="show-jobs">jobs</InternalLink> table. More schema changes create more jobs, which we can delete faster to make internal job queries faster.                                                               |
| `jobs.registry.interval.cancel`                              | `180s`  | Timing of an internal task that queries the <InternalLink path="show-jobs">jobs</InternalLink> table. For testing, the default is too fast.                                                                                                                                       |
| `jobs.retention_time`                                        | `15s`   | More <InternalLink path="online-schema-changes">schema changes</InternalLink> create more <InternalLink path="show-jobs">jobs</InternalLink>, which affects job query performance. We don’t need to retain jobs during testing and can set a more aggressive delete policy.       |
| `sql.stats.automatic_collection.enabled`                     | `false` | Turn off <InternalLink path="show-statistics">statistics</InternalLink> collection, since automatic statistics contribute to table contention alongside schema changes. Each schema change triggers an asynchronous auto statistics job.                                          |
| `ALTER RANGE default CONFIGURE ZONE USING "gc.ttlseconds"`   | `600`   | Faster descriptor cleanup. For more information, see <InternalLink path="alter-range#configure-zone">`ALTER RANGE ... CONFIGURE ZONE`</InternalLink>.                                                                                                                             |
| `ALTER DATABASE system CONFIGURE ZONE USING "gc.ttlseconds"` | `600`   | Faster jobs table cleanup. For more information, see <InternalLink path="alter-database#configure-zone">`ALTER DATABASE ... CONFIGURE ZONE`</InternalLink>.                                                                                                                       |

To change all of the settings described above at once, run the following SQL statements:

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
SET CLUSTER SETTING kv.range_merge.queue_interval = '50ms';
SET CLUSTER SETTING jobs.registry.interval.gc = '30s';
SET CLUSTER SETTING jobs.registry.interval.cancel = '180s';
SET CLUSTER SETTING jobs.retention_time = '15s';
SET CLUSTER SETTING sql.stats.automatic_collection.enabled = false;
SET CLUSTER SETTING kv.range_split.by_load_merge_delay = '5s';
ALTER RANGE default CONFIGURE ZONE USING "gc.ttlseconds" = 600;
ALTER DATABASE system CONFIGURE ZONE USING "gc.ttlseconds" = 600;
```

<Danger>
  These settings **are not** recommended for performance benchmarking of CockroachDB since they will lead to inaccurate results.
</Danger>

## Scope tests to a database when possible

It is better to scope tests to a <InternalLink path="create-database">database</InternalLink> than to a <InternalLink path="create-schema">user-defined schema</InternalLink> due to inefficient user-defined schema validation. The performance of user-defined schema validation may be improved in a future release.

## Log test output to a file

By default, `cockroach start-single-node` logs cluster activity to a file with the <InternalLink path="configure-logs#default-logging-configuration">default logging configuration</InternalLink>. When you specify the `--store=type=mem` flag, the command prints cluster activity directly to the console instead.

To customize logging behavior for local clusters, use the <InternalLink path="cockroach-start-single-node#logging">`--log` flag</InternalLink>:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
cockroach start-single-node --insecure --store=type=mem,size=0.25 --advertise-addr=localhost --log="{file-defaults: {dir: /path/to/logs}, sinks: {stderr: {filter: NONE}}}"
```

The `log` flag has two suboptions:

* `file-defaults`, which specifies the path of the file in which to log events (`/path/to/logs`).
* `sinks`, which provides a secondary destination to which to log events (`stderr`).

For more information about logging, see <InternalLink path="configure-logs">Configure logs</InternalLink>.

## Use a local file server for bulk operations

To test bulk operations like <InternalLink path="backup">`BACKUP`</InternalLink> or <InternalLink path="restore">`RESTORE`</InternalLink>, we recommend using a local file server.

For more details, see <InternalLink path="use-a-local-file-server">Use a Local File Server</InternalLink>.

## Use Docker-specific testing and development tools

When you use the `cockroach start-single-node` command to start a single-node cluster with Docker, some additional features are available to help with testing and development. Refer to <InternalLink path="start-a-local-cluster-in-docker-linux">Start a local cluster in Docker (Linux)</InternalLink> and <InternalLink path="start-a-local-cluster-in-docker-mac">Start a local cluster in Docker (macOS)</InternalLink>.

## See also

* <InternalLink path="query-behavior-troubleshooting">Troubleshoot SQL Statements</InternalLink>
* <InternalLink path="make-queries-fast">Optimize Statement Performance</InternalLink>

