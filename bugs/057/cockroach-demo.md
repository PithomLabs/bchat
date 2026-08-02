> ## Documentation Index
> Fetch the complete documentation index at: https://www.cockroachlabs.com/llms.txt
> Use this file to discover all available pages before exploring further.

# cockroach demo

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

The `cockroach demo` <InternalLink path="cockroach-commands">command</InternalLink> starts a temporary, in-memory CockroachDB cluster of one or more nodes, with or without a preloaded dataset, and opens an interactive SQL shell to the cluster.

* All [SQL shell](#sql-shell) commands, client-side options, help, and shortcuts supported by the <InternalLink path="cockroach-sql#sql-flag-format">`cockroach sql`</InternalLink> command are also supported by `cockroach demo`.
* The in-memory cluster persists only as long as the SQL shell is open. As soon as the shell is exited, the cluster and all its data are permanently destroyed. This command is therefore recommended only as an easy way to experiment with the CockroachDB SQL dialect.
* By default, `cockroach demo` starts in secure mode using TLS certificates to encrypt network communication. It also serves a local [DB Console](#connection-parameters) that does not use TLS encryption.
* Each instance of `cockroach demo` loads a temporary [Enterprise license](https://www.cockroachlabs.com/get-cockroachdb/enterprise/) that expires after 24 hours. To prevent the loading of a temporary license, set the `--disable-demo-license` flag.
* `cockroach demo` opens the SQL shell with a new <InternalLink path="security-reference/authorization#sql-users">SQL user</InternalLink> named `demo`. The `demo` user is assigned a random password and granted the <InternalLink path="security-reference/authorization#admin-role">`admin` role</InternalLink>.

<Danger>
  `cockroach demo` is designed for testing purposes only. It is not suitable for production deployments. To see a list of recommendations for production deployments, see the <InternalLink path="recommended-production-settings">Production Checklist</InternalLink>.
</Danger>

## Synopsis

View help for `cockroach demo`:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
$ cockroach demo --help
```

Start a single-node demo cluster with the `movr` dataset pre-loaded:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
$ cockroach demo <flags>
```

Load a different dataset into a demo cluster:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
$ cockroach demo <dataset> <flags>
```

Run the `movr` workload against a demo cluster:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
$ cockroach demo --with-load <other flags>
```

Execute SQL from the command line against a demo cluster:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
$ cockroach demo --execute="<sql statement;<sql statement" --execute="<sql-statement" <other flags>
```

Start a multi-node demo cluster:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
$ cockroach demo --nodes=<number of nodes <other flags>
```

Start a multi-region demo cluster with default region and zone localities:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
$ cockroach demo --global --nodes=<number of nodes
```

Start a multi-region demo cluster with manually defined localities:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
$ cockroach demo --nodes=<number of nodes --demo-locality=<key:value pair per node <other flags>
```

Stop a demo cluster:

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
> \q
```

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
> quit
```

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
> exit
```

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ctrl-d
```

## Datasets

<Tip>
  By default, the `movr` dataset is pre-loaded into a demo cluster. To load a different dataset, use [`cockroach demo <dataset>`](#load-a-sample-dataset-into-a-demo-cluster). To start a demo cluster without a pre-loaded dataset, pass the `--no-example-database` flag.
</Tip>

| Workload   | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bank`     | A `bank` database, with one `bank` table containing account details.                                                                                                                                                                                                                                                                                                                                                                                                                  |
| `intro`    | An `intro` database, with one table, `mytable`, with a hidden message.                                                                                                                                                                                                                                                                                                                                                                                                                |
| `kv`       | A `kv` database, with one key-value-style table.                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `movr`     | A `movr` database, with several tables of data for the <InternalLink path="movr">MovR example application</InternalLink>.<br /><br />By default, `cockroach demo` loads the `movr` database as the <InternalLink path="sql-name-resolution#current-database">current database</InternalLink>, with sample region (`region`) and availability zone (`az`) replica localities for each node specified with the <InternalLink path="cockroach-demo#flags">`--nodes` flag</InternalLink>. |
| `startrek` | A `startrek` database, with two tables, `episodes` and `quotes`.                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `tpcc`     | A `tpcc` database, with a rich schema of multiple tables.                                                                                                                                                                                                                                                                                                                                                                                                                             |
| `ycsb`     | A `ycsb` database, with a `usertable` from the Yahoo! Cloud Serving Benchmark.                                                                                                                                                                                                                                                                                                                                                                                                        |

## Flags

### General

The `demo` command supports the following general-use flags.

| Flag                                                             | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| ---------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `--auto-enable-rangefeeds`                                       | Override the default behavior of `cockroach demo`, which has rangefeeds enabled on startup.  If you do not need to use <InternalLink path="create-and-configure-changefeeds#enable-rangefeeds">changefeeds</InternalLink> with your demo cluster, use `--auto-enable-rangefeeds=false` to disable rangefeeds and improve performance. See <InternalLink path="create-and-configure-changefeeds#enable-rangefeeds">Enable rangefeeds</InternalLink> for more detail. <br /><br />**Default:** `true`                                                                                                                                                                                                                                         |
| `--cache`                                                        | For each demo node, the total size for caches. This can be a percentage (notated as a decimal or with `%`) or any bytes-based unit, for example: <br /><br />`--cache=.25`<br />`--cache=25%`<br />`--cache=1000000000 ----> 1000000000 bytes`<br />`--cache=1GB ----> 1000000000 bytes`<br />`--cache=1GiB ----> 1073741824 bytes` <br /><br />**Default:** `64MiB`                                                                                                                                                                                                                                                                                                                                                                        |
| <a id="demo-locality" /> `--demo-locality`                       | Specify <InternalLink path="cockroach-start#locality">locality</InternalLink> information for each demo node. The input is a colon-separated list of key-value pairs, where the <i>ii</i><sup>th</sup> demo cockroach node.<br /><br />For example, the following option assigns node 1's region to `us-east1` and availability zone to `1`, node 2's region to `us-east2` and availability zone to `2`, and node 3's region to `us-east3` and availability zone to `3`:<br /><br />`--demo-locality=region=us-east1,az=1:region=us-east1,az=2:region=us-east1,az=3`<br /><br />By default, `cockroach demo` uses sample region (`region`) and availability zone (`az`) replica localities for each node specified with the `--nodes` flag. |
| `--disable-demo-license`                                         | Start the demo cluster without loading a temporary [Enterprise license](https://www.cockroachlabs.com/get-started-cockroachdb/) that expires after 24 hours.<br /><br />Setting the `COCKROACH_SKIP_ENABLING_DIAGNOSTIC_REPORTING` environment variable will also prevent the loading of a temporary license, along with preventing the sharing of anonymized <InternalLink path="diagnostics-reporting">diagnostic details</InternalLink> with Cockroach Labs.                                                                                                                                                                                                                                                                             |
| `--echo-sql`                                                     | Reveal the SQL statements sent implicitly by the command-line utility. This can also be enabled within the interactive SQL shell via the `\set echo` [shell command](#commands).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| `--embedded`                                                     | Minimizes the SQL shell [welcome text](#welcome-text) to be appropriate for embedding in playground-type environments. Specifically, this flag removes details that users in an embedded environment have no control over (e.g., networking information).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| `--no-example-database`                                          | Start the demo cluster without a pre-loaded dataset.<br />To obtain this behavior automatically in every new `cockroach demo` session, set the `COCKROACH_NO_EXAMPLE_DATABASE` environment variable to `true`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| <a id="sql-flag-execute" /> `--execute`<br /><br />`-e`          | Execute SQL statements directly from the command line, without opening a shell. This flag can be set multiple times, and each instance can contain one or more statements separated by semi-colons.<br /><br />If an error occurs in any statement, the command exits with a non-zero status code and further statements are not executed. The results of each statement are printed to the standard output (see `--format` for formatting options).                                                                                                                                                                                                                                                                                        |
| <a id="sql-flag-format" /> `--format`                            | How to display table rows printed to the standard output. Possible values: `tsv`, `csv`, `table`, `raw`, `records`, `sql`, `html`.<br /><br />**Default:** `table` for sessions that <InternalLink path="cockroach-sql#session-and-output-types">output on a terminal</InternalLink>; `tsv` otherwise<br /><br />This flag corresponds to the `display_format` [client-side option](#client-side-options) for use in interactive sessions.                                                                                                                                                                                                                                                                                                  |
| <a id="geo-partitioned-replicas" /> `--geo-partitioned-replicas` | Start a 9-node demo cluster with <InternalLink path="partitioning">geo-partitioning</InternalLink> applied to the <InternalLink path="movr">`movr`</InternalLink> database.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| <a id="global-flag" /> `--global`                                | Simulates a <InternalLink path="simulate-a-multi-region-cluster-on-localhost">multi-region cluster</InternalLink> which sets the <InternalLink path="cockroach-start#locality">`--locality` flag on node startup</InternalLink> to three different regions. It also simulates the network latency that would occur between them given the specified localities. In order for this to operate as expected, with 3 nodes in each of 3 regions, you must also pass the `--nodes 9` argument.                                                                                                                                                                                                                                                   |
| `--http-port`                                                    | Specifies a custom HTTP port to the <InternalLink path="ui-overview">DB Console</InternalLink> for the first node of the demo cluster.<br /><br />In multi-node clusters, the HTTP ports for additional clusters increase from the port of the first node, in increments of 1. For example, if the first node has an HTTP port of `5000`, the second node will have the HTTP port `5001`.                                                                                                                                                                                                                                                                                                                                                   |
| `--insecure`                                                     | Include this to start the demo cluster in insecure mode.<br /><br />**Env Variable:** `COCKROACH_INSECURE`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| `--listening-url-file`                                           | The file to which the node's SQL connection URL will be written as soon as the demo cluster is initialized and the node is ready to accept connections. <br /><br />This flag is useful for automation because it allows you to wait until the demo cluster has been initialized so that subsequent commands can connect automatically.                                                                                                                                                                                                                                                                                                                                                                                                     |
| `--max-sql-memory`                                               | For each demo node, the maximum in-memory storage capacity for temporary SQL data, including prepared queries and intermediate data rows during query execution. This can be a percentage (notated as a decimal or with `%`) or any bytes-based unit, for example:<br /><br />`--max-sql-memory=.25`<br />`--max-sql-memory=25%`<br />`--max-sql-memory=10000000000 ----> 1000000000 bytes`<br />`--max-sql-memory=1GB ----> 1000000000 bytes`<br />`--max-sql-memory=1GiB ----> 1073741824 bytes`<br /><br />**Default:** `128MiB`                                                                                                                                                                                                         |
| `--nodes`                                                        | Specify the number of in-memory nodes to create for the demo.<br /><br />**Default:** 1                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| <a id="flag-safe-updates" /> `--safe-updates`                    | Disallow potentially unsafe SQL statements, including `DELETE` without a `WHERE` clause, `UPDATE` without a `WHERE` clause, and `ALTER TABLE ... DROP COLUMN`.<br /><br />**Default:** `true` for <InternalLink path="cockroach-sql#session-and-output-types">interactive sessions</InternalLink>; `false` otherwise<br /><br />Potentially unsafe SQL statements can also be allowed/disallowed for an entire session via the `sql_safe_updates` <InternalLink path="set-vars">session variable</InternalLink>.                                                                                                                                                                                                                            |
| `--set`                                                          | Set a [client-side option](#client-side-options) before starting the SQL shell or executing SQL statements from the command line via `--execute`. This flag may be specified multiple times, once per option.<br /><br />After starting the SQL shell, the `\set` and `unset` commands can be use to enable and disable client-side options as well.                                                                                                                                                                                                                                                                                                                                                                                        |
| `--sql-port`                                                     | Specifies a custom SQL port for the first node of the demo cluster.<br /><br />In multi-node clusters, the SQL ports for additional clusters increase from the port of the first node, in increments of 1. For example, if the first node has the SQL port `3000`, the second node will the SQL port `3001`.                                                                                                                                                                                                                                                                                                                                                                                                                                |
| `--with-load`                                                    | Run a demo <InternalLink path="movr">`movr`</InternalLink> workload against the preloaded `movr` database.<br /><br /> When running a multi-node demo cluster, load is balanced across all nodes.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |

### Logging

By default, the `demo` command does not log messages.

If you need to troubleshoot this command's behavior, you can <InternalLink path="configure-logs">customize its logging behavior</InternalLink>.

## SQL shell

### Welcome text

When the SQL shell connects to the demo cluster at startup, it prints a welcome text with some tips and cluster details. Most of these details resemble the <InternalLink path="cockroach-sql#welcome-message">welcome text</InternalLink> that is printed when connecting `cockroach sql` to a permanent cluster. `cockroach demo` also includes some [connection parameters](#connection-parameters) for connecting to the DB Console or for connecting another SQL client to the demo cluster.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
#
# Welcome to the CockroachDB demo database!
#
# You are connected to a temporary, in-memory CockroachDB cluster of 9 nodes.
#
# Beginning initialization of the movr dataset, please wait...
#
# Waiting for license acquisition to complete...
#
# Partitioning the demo database, please wait...
#
# The cluster has been preloaded with the "movr" dataset
# (MovR is a fictional vehicle sharing company).
#
# Reminder: your changes to data stored in the demo session will not be saved!
#
# If you wish to access this demo cluster using another tool, you will need
# the following details:
#
#   - Connection parameters:
#     (webui)    http://127.0.0.1:8080/demologin?password=demo55826&username=demo
#     (sql)      postgresql://demo:demo55826@127.0.0.1:26257/movr?sslmode=require
#     (sql/jdbc) jdbc:postgresql://127.0.0.1:26257/movr?password=demo55826&sslmode=require&user=demo
#     (sql/unix) postgresql://demo:demo55826@/movr?host=%2Fvar%2Ffolders%2F8c%2F915dtgrx5_57bvc5tq4kpvqr0000gn%2FT%2Fdemo699845497&port=26257
#
#   To display connection parameters for other nodes, use \demo ls.
#   - Username: "demo", password: "demo55826"
#   - Directory with certificate files (for certain SQL drivers/tools): /var/folders/8c/915dtgrx5_57bvc5tq4kpvqr0000gn/T/demo699845497
#
# Server version: CockroachDB CCL  (x86_64-apple-darwin20.5.0, built ) (same version as client)
# Cluster ID: f78b7feb-b6cf-4396-9d7f-494982d7d81e
# Organization: Cockroach Demo
#
# Enter \? for a brief introduction.
#
```

### Connection parameters

The SQL shell welcome text includes connection parameters for accessing the DB Console and for connecting other SQL clients to the demo cluster:

```
#   - Connection parameters:
#     (webui)    http://127.0.0.1:8080/demologin?password=demo55826&username=demo
#     (sql)      postgresql://demo:demo55826@127.0.0.1:26257/movr?sslmode=require
#     (sql/jdbc) jdbc:postgresql://127.0.0.1:26257/movr?password=demo55826&sslmode=require&user=demo
#     (sql/unix) postgresql://demo:demo55826@/movr?host=%2Fvar%2Ffolders%2F8c%2F915dtgrx5_57bvc5tq4kpvqr0000gn%2FT%2Fdemo699845497&port=26257
```

| Parameter  | Description                                                                                                                                                                                                                                           |
| ---------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `webui`    | Use this link to access a local <InternalLink path="ui-overview">DB Console</InternalLink> to the demo cluster.                                                                                                                                       |
| `sql`      | Use this connection URL for standard sql/tcp connections from other SQL clients such as <InternalLink path="cockroach-sql#sql-flag-format">`cockroach sql`</InternalLink>.<br />The default SQL port for the first node of a demo cluster is `26257`. |
| `sql/unix` | Use this connection URL to establish a <InternalLink path="cockroach-sql#connect-to-a-cluster-listening-for-unix-domain-socket-connections">Unix domain socket connection</InternalLink> with a client that is installed on the same machine.         |

<Note>
  You do not need to create or specify node and client certificates in `sql` or `sql/unix` connection URLs. Instead, you can securely connect to the demo cluster with the random password generated for the `demo` user.
</Note>

When running a multi-node demo cluster, use the `\demo ls` [shell command](#commands) to list the connection parameters for all nodes:

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
> \demo ls
```

```
node 1:
  (webui)    http://127.0.0.1:8080/demologin?password=demo76950&username=demo
  (sql)      postgres://demo:demo76950@127.0.0.1:26257?sslmode=require
  (sql/unix) postgres://demo:demo76950@?host=%2Fvar%2Ffolders%2Fc8%2Fb_q93vjj0ybfz0fz0z8vy9zc0000gp%2FT%2Fdemo070856957&port=26257

node 2:
  (webui)    http://127.0.0.1:8081/demologin?password=demo76950&username=demo
  (sql)      postgres://demo:demo76950@127.0.0.1:26258?sslmode=require
  (sql/unix) postgres://demo:demo76950@?host=%2Fvar%2Ffolders%2Fc8%2Fb_q93vjj0ybfz0fz0z8vy9zc0000gp%2FT%2Fdemo070856957&port=26258

node 3:
  (webui)    http://127.0.0.1:8082/demologin?password=demo76950&username=demo
  (sql)      postgres://demo:demo76950@127.0.0.1:26259?sslmode=require
  (sql/unix) postgres://demo:demo76950@?host=%2Fvar%2Ffolders%2Fc8%2Fb_q93vjj0ybfz0fz0z8vy9zc0000gp%2FT%2Fdemo070856957&port=26259
```

### Commands

* [General](#general)
* [Demo-specific](#demo-specific)

#### General

The following commands can be used within the interactive SQL shell:

| Command                                            | Usage                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| -------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `\?`,`help`                                        | View this help within the shell.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| `\q`,`quit`,`exit`,`ctrl-d`                        | Exit the shell.<br />When no text follows the prompt, `ctrl-c` exits the shell as well; otherwise, `ctrl-c` clears the line.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| `\!`                                               | Run an external command and print its results to `stdout`. <InternalLink path="cockroach-sql#run-external-commands-from-the-sql-shell">See an example</InternalLink>.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `\|`                                               | Run the output of an external command as SQL statements. <InternalLink path="cockroach-sql#run-external-commands-from-the-sql-shell">See an example</InternalLink>.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| `\set <option>`,`\unset <option>`                  | Enable or disable a client-side option. For more details, see [Client-side options](#client-side-options).<br />You can also use the [`--set` flag](#general) to enable or disable client-side options before starting the SQL shell.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `\p`,`\show`                                       | During a multi-line statement or transaction, show the SQL that has been entered but not yet executed.<br />`\show` was deprecated as of v21.1. Use `\p` instead.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| `\h <statement>`,`\hf <function>`                  | View help for specific SQL statements or functions. See [SQL shell help](#help) for more details.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| `\c <option>`,`\connect <option>`                  | Display or change the current <InternalLink path="connection-parameters">connection parameters</InternalLink>. Using `\c` without an argument lists the current connection parameters. <br />To reuse the existing connection and change the current database, use `\c <dbname>`. This is equivalent to `SET <database>` and `USE <database>`. <br />To connect to a cluster using individual connection parameters, use `\c <dbname> <user> <host> <port>`. Use the dash character (`-`) to omit one parameter. To reconnect to the cluster using the current connection parameters enter `\c -`. When using individual connection parameters, the TLS settings from the original connection are reused. To use different TLS settings, connect using a connection URL. <br />To connect to a cluster using a <InternalLink path="connection-parameters#connect-using-a-url">connection URL</InternalLink> use `\c <url>` |
| `\l`                                               | List all databases in the CockroachDB cluster. This command is equivalent to <InternalLink path="show-databases">`SHOW DATABASES`</InternalLink>.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| `\d[S+] [<pattern]`                                | Show details about the relations in the current database. By default this command will show all the user tables, indexes, views, materialized views, and sequences in the current database. Add the `S` modifier to also show all system objects. If you specify a relation or a [pattern](#patterns), it will show the details of matching relations. Add the `+` modifier to show additional information.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| `\dC[+] [<pattern]`                                | Show the type casts. If you specify a type or a [pattern](#patterns), it will show the details of matching types. Add the `+` modifier to show additional information.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| `\dd[S] [<pattern]`                                | Show the objects of type `constraint` in the current database. Add the `S` modifier to also show all system objects. If you specify a type or a [pattern](#patterns), it will show the details of matching objects.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| `\df[S+] [<pattern]`                               | Show the <InternalLink path="user-defined-functions">user-defined functions</InternalLink> of the current database. Add the `S` modifier to also show all <InternalLink path="functions-and-operators">system functions</InternalLink>. If you specify a function name or a [pattern](#patterns), it will show the details of matching function. Add the `+` modifier to show additional information.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `\dg[S+] [<pattern]`                               | Show the <InternalLink path="authorization">roles</InternalLink> of the current database. Add the `S` modifier to also show all system objects. If you specify a role name or a [pattern](#patterns), it will show the details of matching roles. Add the `+` modifier to show additional information.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| `\di[S+] [<pattern]`                               | Show the indexes of the current database. Add the `S` modifier to also show all system objects. If you specify an index name or a [pattern](#patterns), it will show the details of matching indexes. Add the `+` modifier to show additional information.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| `\dm[S+] [<pattern]`                               | Show the materialized views of the current database. Add the `S` modifier to also show all system objects. If you specify a materialized view name or a [pattern](#patterns), it will show the details of matching materialized views. Add the `+` modifier to show additional information.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| `\dn[S+] [<pattern]`                               | List all <InternalLink path="sql-name-resolution#naming-hierarchy">schemas</InternalLink> in the current database. Add the `S` modifier to also show all system schemas. Add the `+` modifier to show the permissions of each schema. Specify a [pattern](#patterns) to limit the output to schemas that match the pattern. These commands are equivalent to <InternalLink path="show-schemas">`SHOW SCHEMAS`</InternalLink>.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| `\ds[S+] [<pattern]`                               | Show the sequences of the current database. Add the `S` modifier to also show all system objects. If you specify a sequence name or a [pattern](#patterns), it will show the details of matching sequences. Add the `+` modifier to show additional information.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| `\dt[S+] [<pattern]`                               | Show the tables of the current database. Add the `S` modifier to also show all system objects. If you specify a table name or a [pattern](#patterns), it will show the details of matching tables. Add the `+` modifier to show additional information.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| `\dT[S+] [<pattern]`                               | Show the <InternalLink path="enum">user-defined types</InternalLink> in the current database. Add the `S` modifier to also show all system objects. If you specify a type name or a [pattern](#patterns), it will show the details of matching types. Add the `+` modifier to show additional information.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| `\du[S+] [<pattern]`                               | Show the <InternalLink path="authorization">roles</InternalLink> of the current database. Add the `S` modifier to also show all system objects. If you specify a role name or a [pattern](#patterns), it will show the details of matching roles. Add the `+` modifier to show additional information.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| `\dv[S+] [<pattern]`                               | Show the views of the current database. Add the `S` modifier to also show all system objects. If you specify a view name or a [pattern](#patterns), it will show the details of matching views. Add the `+` modifier to show additional information.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| `\r`                                               | Resets the query input buffer, clearing all SQL statements that have been entered but not yet executed.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| `\statement-diag list`                             | List available <InternalLink path="cockroach-statement-diag">diagnostic bundles</InternalLink>.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| `\statement-diag download <bundle-id> [<filename]` | Download diagnostic bundle.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| `\i <filename>`                                    | Reads and executes input from the file `<filename`, in the current working directory.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `\ir <filename>`                                   | Reads and executes input from the file `<filename`.<br />When invoked in the interactive shell, `\i` and `\ir` behave identically (i.e., CockroachDB looks for `<filename` in the current working directory). When invoked from a script, CockroachDB looks for `<filename` relative to the directory in which the script is located.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `\echo <arguments>`                                | Evaluate the `<arguments` and print the results to the standard output.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| `\x <boolean>`                                     | When `true`/`on`/`yes`/`1`, <InternalLink path="cockroach-sql#sql-flag-format">sets the display format</InternalLink> to `records`. When `false`/`off`/`no`/`0`, sets the session's format to the default (`table`/`tsv`).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |

#### Patterns

Commands use the SQL <InternalLink path="scalar-expressions#string-pattern-matching">`LIKE` syntax</InternalLink> for string pattern matching, not POSIX regular expressions.

For example to list all schemas that begin with the letter "p" you'd use the following pattern:

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
\dn p%
```

```
List of schemas:
      Name     | Owner
---------------+--------
  pg_catalog   | NULL
  pg_extension | NULL
  public       | admin
(3 rows)
```

#### Demo-specific

`cockroach demo` offers the following additional shell commands. Note that these commands are **experimental** and their interface and output are subject to change.

| Command                               | Usage                                                                                                                                                                                                     |
| ------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `\demo ls`                            | List the demo nodes and their connection URLs.                                                                                                                                                            |
| `\demo add region=<region,zone=<zone` | Add a node to a single-region or multi-region demo cluster. [See an example](#add-shut-down-and-restart-nodes-in-a-multi-node-demo-cluster).                                                              |
| `\demo shutdown <node number>`        | Shuts down a node in a multi-node demo cluster.<br /><br />This command simulates stopping a node that can be restarted. [See an example](#add-shut-down-and-restart-nodes-in-a-multi-node-demo-cluster). |
| `\demo restart <node number>`         | Restarts a node in a multi-node demo cluster. [See an example](#add-shut-down-and-restart-nodes-in-a-multi-node-demo-cluster).                                                                            |
| `\demo decommission <node number>`    | Decommissions a node in a multi-node demo cluster.<br /><br />This command simulates <InternalLink path="node-shutdown?filters=decommission">decommissioning a node</InternalLink>.                       |
| `\demo recommission <node number>`    | Recommissions a decommissioned node in a multi-node demo cluster.                                                                                                                                         |

### Client-side options

* To view option descriptions and how they are currently set, use `\set` without any options.
* To enable or disable an option, use `\set <option> <value>` or `\unset <option> <value>`. You can also use the form `<option=<value`.
* If an option accepts a boolean value:
  * `\set <option>` without `<value` is equivalent to `\set <option> true`, and `\unset <option>` without `<value` is equivalent to `\set <option> false`.
  * `on`, `yes`, and `1` are aliases for `true`, and `off`, `no`, and `0` are aliases for `false`.

| Client Options                                        | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| ----------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| <a id="sql-option-auto-trace" /> `auto_trace`         | For every statement executed, the shell also produces the trace for that statement in a separate result below. A trace is also produced in case the statement produces a SQL error.<br /><br />**Default:** `off`<br /><br />To enable this option, run `\set auto_trace on`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| <a id="sql-option-border" /> `border`                 | Display a border around the output of the SQL statement when using the `table` display format. Set the level of borders using `border=<level` to configure how many borders and lines are in the output, where `<level` is an integer between 0 and 3. The higher the integer, the more borders and lines are in the output. <br />A level of `0` shows the output with no outer lines and no row line separators. <br />A level of `1` adds row line separators. A level of `2` adds an outside border and no row line separators. A level of `3` adds both an outside border and row line separators. <br /><br />**Default:** `0` <br /><br />To change this option, run `\set border=<level`. <InternalLink path="cockroach-sql#show-borders-around-the-statement-output-within-the-sql-shell">See an example</InternalLink>. |
| <a id="sql-option-check-syntax" /> `check_syntax`     | Validate SQL syntax. This ensures that a typo or mistake during user entry does not inconveniently abort an ongoing transaction previously started from the interactive shell.<br /><br />**Default:** `true` for <InternalLink path="cockroach-sql#session-and-output-types">interactive sessions</InternalLink>; `false` otherwise.<br /><br />To disable this option, run `\unset check_syntax`.                                                                                                                                                                                                                                                                                                                                                                                                                               |
| <a id="sql-option-display-format" /> `display_format` | How to display table rows printed within the interactive SQL shell. Possible values: `tsv`, `csv`, `table`, `raw`, `records`, `sql`, `html`.<br /><br />**Default:** `table` for sessions that <InternalLink path="cockroach-sql#session-and-output-types">output on a terminal</InternalLink>; `tsv` otherwise<br /><br />To change this option, run `\set display_format <format>`. <InternalLink path="cockroach-sql#make-the-output-of-show-statements-selectable">See an example</InternalLink>.                                                                                                                                                                                                                                                                                                                             |
| <a id="sql-option-echo" /> `echo`                     | Reveal the SQL statements sent implicitly by the SQL shell.<br /><br />**Default:** `false`<br /><br />To enable this option, run `\set echo`. <InternalLink path="cockroach-sql#reveal-the-sql-statements-sent-implicitly-by-the-command-line-utility">See an example</InternalLink>.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| <a id="sql-option-errexit" /> `errexit`               | Exit the SQL shell upon encountering an error.<br /><br />**Default:** `false` for <InternalLink path="cockroach-sql#session-and-output-types">interactive sessions</InternalLink>; `true` otherwise<br /><br />To enable this option, run `\set errexit`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| <a id="sql-option-prompt1" /> `prompt1`               | Customize the interactive prompt within the SQL shell. See [Customizing the prompt](#customizing-the-prompt) for information on the available prompt variables.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| <a id="sql-option-show-times" /> `show_times`         | Reveal the time a query takes to complete. Possible values:<br />                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |

* `execution` time refers to the time taken by the SQL execution engine to execute the query.
* `network` time refers to the network latency between the server and the SQL client command.
* `other` time refers to all other forms of latency affecting the total query completion time, including query planning.

**Default:** `true`<br /><br />To disable this option, run `\unset show_times`.

#### Customizing the prompt

The `\set prompt1` option allows you to customize the interactive prompt in the SQL shell. Use the following prompt variables to set a custom prompt.

| Prompt variable | Description                                         |
| --------------- | --------------------------------------------------- |
| `%>`            | The port of the node you are connected to.          |
| `%/`            | The current database name.                          |
| `%M`            | The fully qualified host name and port of the node. |
| `%m`            | The fully qualified host name of the node.          |
| `%n`            | The username of the connected SQL user.             |
| `%x`            | The transaction status of the current statement.    |

For example, to change the prompt to just the user, host, and database:

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
\set prompt1 %n@%m/%/
```

```
maxroach@blue-dog-595.g95.cockroachlabs.cloud/defaultdb>
```

### Help

Within the SQL shell, you can get interactive help about statements and functions:

| Command                                    | Usage                                                    |
| ------------------------------------------ | -------------------------------------------------------- |
| `\h`<br /><br />`??`                       | List all available SQL statements, by category.          |
| `\hf`                                      | List all available SQL functions, in alphabetical order. |
| `\h <statement>`<br /><br />`<statement ?` | View help for a specific SQL statement.                  |
| `\hf <function>`<br /><br />`<function ?`  | View help for a specific SQL function.                   |

<a id="locality-example" />

#### Examples

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
> \h UPDATE
```

```
Command:     UPDATE
Description: update rows of a table
Category:    data manipulation
Syntax:
UPDATE <tablename> [[AS] <name>] SET ... [WHERE <expr>] [RETURNING <exprs...>]

See also:
  SHOW TABLES
  INSERT
  UPSERT
  DELETE
  https://www.cockroachlabs.com/docs/v26.2/update.html
```

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
> \hf uuid_v4
```

```
Function:    uuid_v4
Category:    built-in functions
Returns a UUID.

Signature          Category
uuid_v4() -> bytes [ID Generation]

See also:
  https://www.cockroachlabs.com/docs/v26.2/functions-and-operators.html
```

### Shortcuts

Note: macOS users may need to manually enable Alt-based shortcuts in their terminal configuration. See the section [macOS terminal configuration](#macos-terminal-configuration) below for details.

| Shortcut                | Description                                                                        |
| ----------------------- | ---------------------------------------------------------------------------------- |
| Tab                     | Use [context-sensitive command completion](#tab-completion).                       |
| Ctrl+C                  | Clear/cancel the input.                                                            |
| Ctrl+M, Enter           | New line/enter.                                                                    |
| Ctrl+O                  | Force a new line on the current statement, even if the statement has a semicolon.  |
| Ctrl+F, Right arrow     | Forward one character.                                                             |
| Ctrl+B, Left arrow      | Backward one character.                                                            |
| Alt+F, Ctrl+Right arrow | Forward one word.                                                                  |
| Alt+B, Ctrl+Left arrow  | Backward one word.                                                                 |
| Ctrl+L                  | Refresh the display.                                                               |
| Delete                  | Delete the next character.                                                         |
| Ctrl+H, Backspace       | Delete the previous character.                                                     |
| Ctrl+D                  | Delete the next character, or terminate the input if the input is currently empty. |
| Alt+D, Alt+Delete       | Delete next word.                                                                  |
| Ctrl+W, Alt+Backspace   | Delete previous word.                                                              |
| Ctrl+E, End             | End of line.                                                                       |
| Alt+>, Ctrl+End         | Move cursor to the end of a multi-line statement.                                  |
| Ctrl+A, Home            | Move cursor to the beginning of the current line.                                  |
| Alt+\<, Ctrl+Home       | Move cursor to the beginning of a multi-line statement.                            |
| Ctrl+T                  | Transpose current and next characters.                                             |
| Ctrl+K                  | Delete from cursor position until end of line.                                     |
| Ctrl+U                  | Delete from beginning of line to cursor position.                                  |
| Alt+Q                   | Reflow/reformat the current line.                                                  |
| Alt+Shift+Q, Alt+\`     | Reflow/reformat the entire input.                                                  |
| Alt+L                   | Convert the current word to lowercase.                                             |
| Alt+U                   | Convert the current word to uppercase.                                             |
| Alt+.                   | Toggle the visibility of the prompt.                                               |
| Alt+2, Alt+F2           | Invoke external editor on current input.                                           |
| Alt+P, Up arrow         | Recall previous history entry.                                                     |
| Alt+N, Down arrow       | Recall next history entry.                                                         |
| Ctrl+R                  | Start searching through input history.                                             |

When searching for history entries, the following shortcuts are active:

| Shortcut       | Description                                        |
| -------------- | -------------------------------------------------- |
| Ctrl+C, Ctrl+G | Cancel the search, return to normal mode.          |
| Ctrl+R         | Recall next entry matching current search pattern. |
| Enter          | Accept the current recalled entry.                 |
| Backspace      | Delete previous character in search pattern.       |
| Other          | Add character to search pattern.                   |

#### Tab completion

The SQL client offers context-sensitive tab completion when entering commands. Use the **Tab** key on your keyboard when entering a command to initiate the command completion interface. You can then navigate to database objects, keywords, and functions using the arrow keys. Press the **Tab** key again to select the object, function, or keyword from the command completion interface and return to the console.

### macOS terminal configuration

In **Apple Terminal**:

1. Navigate to "Preferences", then "Profiles", then "Keyboard".
2. Enable the checkbox "Use Option as Meta Key".

<img src="https://mintcdn.com/cockroachlabs/H2p_QgztDgzpDDkl/images/v26.2/terminal-configuration.png?fit=max&auto=format&n=H2p_QgztDgzpDDkl&q=85&s=6a9c5c09424be025718951e51ffa8cca" alt="Apple Terminal Alt key configuration" width="1536" height="1280" data-path="images/v26.2/terminal-configuration.png" />

In **iTerm2**:

1. Navigate to "Preferences", then "Profiles", then "Keys".
2. Select the radio button "Esc+" for the behavior of the Left Option Key.

<img src="https://mintcdn.com/cockroachlabs/7A5lSB6m0mXfjOZl/images/v26.2/iterm2-configuration.png?fit=max&auto=format&n=7A5lSB6m0mXfjOZl&q=85&s=8ca32e054a9c048586f6367c83a89eea" alt="iTerm2 Alt key configuration" width="2036" height="1164" data-path="images/v26.2/iterm2-configuration.png" />

## Diagnostics reporting

By default, `cockroach demo` shares anonymous usage details with Cockroach Labs. To opt out, set the <InternalLink path="diagnostics-reporting#after-cluster-initialization">`diagnostics.reporting.enabled`</InternalLink> <InternalLink path="cluster-settings">cluster setting</InternalLink> to `false`. You can also opt out by setting the <InternalLink path="diagnostics-reporting#at-cluster-initialization">`COCKROACH_SKIP_ENABLING_DIAGNOSTIC_REPORTING`</InternalLink> environment variable to `false` before running `cockroach demo`.

## Examples

In these examples, we demonstrate how to start a shell with `cockroach demo`. For more SQL shell features, see the <InternalLink path="cockroach-sql#examples">`cockroach sql` examples</InternalLink>.

### Start a single-node demo cluster

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
$ cockroach demo
```

By default, `cockroach demo` loads the `movr` dataset in to the demo cluster:

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
> SHOW TABLES;
```

```
  schema_name |         table_name         | type  | owner | estimated_row_count | locality
--------------+----------------------------+-------+-------+---------------------+-----------
  public      | promo_codes                | table | demo  |                1000 | NULL
  public      | rides                      | table | demo  |                 500 | NULL
  public      | user_promo_codes           | table | demo  |                   0 | NULL
  public      | users                      | table | demo  |                  50 | NULL
  public      | vehicle_location_histories | table | demo  |                1000 | NULL
  public      | vehicles                   | table | demo  |                  15 | NULL
(6 rows)
```

You can query the pre-loaded data:

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
> SELECT name FROM users LIMIT 10;
```

```
         name
-----------------------
  Tyler Dalton
  Dillon Martin
  Deborah Carson
  David Stanton
  Maria Weber
  Brian Campbell
  Carl Mcguire
  Jennifer Sanders
  Cindy Medina
  Daniel Hernandez MD
(10 rows)
```

You can also create and query new tables:

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
> CREATE TABLE drivers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    city STRING NOT NULL,
    name STRING,
    dl STRING UNIQUE,
    address STRING
);
```

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
> INSERT INTO drivers (city, name) VALUES ('new york', 'Catherine Nelson');
```

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
> SELECT * FROM drivers;
```

```
                   id                  |   city   |       name       |  dl  | address
---------------------------------------+----------+------------------+------+----------
  4d363104-2c48-43b5-aa1e-955b81415c7d | new york | Catherine Nelson | NULL | NULL
(1 row)
```

### Start a multi-node demo cluster

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
$ cockroach demo --nodes=3
```

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
> \demo ls
```

```
node 1:
  (webui)    http://127.0.0.1:8080/demologin?password=demo76950&username=demo
  (sql)      postgres://demo:demo76950@127.0.0.1:26257?sslmode=require
  (sql/unix) postgres://demo:demo76950@?host=%2Fvar%2Ffolders%2Fc8%2Fb_q93vjj0ybfz0fz0z8vy9zc0000gp%2FT%2Fdemo070856957&port=26257

node 2:
  (webui)    http://127.0.0.1:8081/demologin?password=demo76950&username=demo
  (sql)      postgres://demo:demo76950@127.0.0.1:26258?sslmode=require
  (sql/unix) postgres://demo:demo76950@?host=%2Fvar%2Ffolders%2Fc8%2Fb_q93vjj0ybfz0fz0z8vy9zc0000gp%2FT%2Fdemo070856957&port=26258

node 3:
  (webui)    http://127.0.0.1:8082/demologin?password=demo76950&username=demo
  (sql)      postgres://demo:demo76950@127.0.0.1:26259?sslmode=require
  (sql/unix) postgres://demo:demo76950@?host=%2Fvar%2Ffolders%2Fc8%2Fb_q93vjj0ybfz0fz0z8vy9zc0000gp%2FT%2Fdemo070856957&port=26259
```

### Load a sample dataset into a demo cluster

By default, `cockroach demo` loads the `movr` dataset in to the demo cluster. To pre-load any of the other [available datasets](#datasets) using `cockroach demo <dataset>`. For example, to load the `ycsb` dataset:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
$ cockroach demo ycsb
```

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
> SHOW TABLES;
```

```
  schema_name | table_name | type  | owner | estimated_row_count | locality
--------------+------------+-------+-------+---------------------+-----------
  public      | usertable  | table | demo  |                   0 | NULL
(1 row)
```

### Run load against a demo cluster

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
$ cockroach demo --with-load
```

This command starts a demo cluster with the `movr` database preloaded and then inserts rows into each table in the `movr` database. You can monitor the workload progress on the <InternalLink path="ui-overview-dashboard#sql-queries-per-second">DB Console</InternalLink>.

When running a multi-node demo cluster, load is balanced across all nodes.

### Execute SQL from the command-line against a demo cluster

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
$ cockroach demo \
--execute="CREATE TABLE drivers (
    id UUID DEFAULT gen_random_uuid(),
    city STRING NOT NULL,
    name STRING,
    dl STRING UNIQUE,
    address STRING,
    CONSTRAINT primary_key PRIMARY KEY (city ASC, id ASC)
);" \
--execute="INSERT INTO drivers (city, name) VALUES ('new york', 'Catherine Nelson');" \
--execute="SELECT * FROM drivers;"
```

```
CREATE TABLE
INSERT 1
                   id                  |   city   |       name       |  dl  | address
---------------------------------------+----------+------------------+------+----------
  dd6afc4c-bf31-455e-bb6d-bfb8f18ad6cc | new york | Catherine Nelson | NULL | NULL
(1 row)
```

### Connect an additional SQL client to the demo cluster

In addition to the interactive SQL shell that opens when you run `cockroach demo`, you can use the [connection parameters](#connection-parameters) in the welcome text to connect additional SQL clients to the cluster.

1. Use `\demo ls` to list the connection parameters for each node in the demo cluster:

   ```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
   \demo ls
   ```

   ```
   node 1:
     (webui)    http://127.0.0.1:8080/demologin?password=demo76950&username=demo
     (sql)      postgres://demo:demo76950@127.0.0.1:26257?sslmode=require
     (sql/unix) postgres://demo:demo76950@?host=%2Fvar%2Ffolders%2Fc8%2Fb_q93vjj0ybfz0fz0z8vy9zc0000gp%2FT%2Fdemo070856957&port=26257

   node 2:
     (webui)    http://127.0.0.1:8081/demologin?password=demo76950&username=demo
     (sql)      postgres://demo:demo76950@127.0.0.1:26258?sslmode=require
     (sql/unix) postgres://demo:demo76950@?host=%2Fvar%2Ffolders%2Fc8%2Fb_q93vjj0ybfz0fz0z8vy9zc0000gp%2FT%2Fdemo070856957&port=26258

   node 3:
     (webui)    http://127.0.0.1:8082/demologin?password=demo76950&username=demo
     (sql)      postgres://demo:demo76950@127.0.0.1:26259?sslmode=require
     (sql/unix) postgres://demo:demo76950@?host=%2Fvar%2Ffolders%2Fc8%2Fb_q93vjj0ybfz0fz0z8vy9zc0000gp%2FT%2Fdemo070856957&port=26259
   ```

2. Open a new terminal and run <InternalLink path="cockroach-sql#sql-flag-format">`cockroach sql`</InternalLink> with the `--url` flag set to the `sql` connection URL of the node to which you want to connect:

   ```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
   $ cockroach sql --url='postgres://demo:demo53628@127.0.0.1:26259?sslmode=require'
   ```

   You can also use this URL to connect an application to the demo cluster as the `demo` user.

### Start a multi-region demo cluster

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
$ cockroach demo --global --nodes 9
```

This command starts a 9-node demo cluster with the `movr` database preloaded and region and zone localities set at the cluster level.

<Note>
  The `--global` flag is an experimental feature of `cockroach demo`. The interface and output are subject to change.
</Note>

### Add, shut down, and restart nodes in a multi-node demo cluster

In a multi-node demo cluster, you can use `\demo` [shell commands](#commands) to add, shut down, restart, decommission, and recommission individual nodes.

<Note>
  **This feature is in <InternalLink version="releases" path="cockroachdb-feature-availability">preview</InternalLink>** and subject to change. To share feedback and/or issues, contact [Support](https://support.cockroachlabs.com).
</Note>

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
$ cockroach demo --nodes=9
```

<Note>
  `cockroach demo` does not support the `\demo add` and `\demo shutdown` commands in demo clusters started with the `--global` flag.
</Note>

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
> SHOW REGIONS FROM CLUSTER;
```

```
     region    |  zones
---------------+----------
  europe-west1 | {b,c,d}
  us-east1     | {b,c,d}
  us-west1     | {a,b,c}
(3 rows)
```

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
> \demo ls
```

```
node 1:
  (webui)    http://127.0.0.1:8080/demologin?password=demo76950&username=demo
  (sql)      postgres://demo:demo76950@127.0.0.1:26257?sslmode=require
  (sql/unix) postgres://demo:demo76950@?host=%2Fvar%2Ffolders%2Fc8%2Fb_q93vjj0ybfz0fz0z8vy9zc0000gp%2FT%2Fdemo070856957&port=26257

node 2:
  (webui)    http://127.0.0.1:8081/demologin?password=demo76950&username=demo
  (sql)      postgres://demo:demo76950@127.0.0.1:26258?sslmode=require
  (sql/unix) postgres://demo:demo76950@?host=%2Fvar%2Ffolders%2Fc8%2Fb_q93vjj0ybfz0fz0z8vy9zc0000gp%2FT%2Fdemo070856957&port=26258

node 3:
  (webui)    http://127.0.0.1:8082/demologin?password=demo76950&username=demo
  (sql)      postgres://demo:demo76950@127.0.0.1:26259?sslmode=require
  (sql/unix) postgres://demo:demo76950@?host=%2Fvar%2Ffolders%2Fc8%2Fb_q93vjj0ybfz0fz0z8vy9zc0000gp%2FT%2Fdemo070856957&port=26259

node 4:
  (webui)    http://127.0.0.1:8083/demologin?password=demo76950&username=demo
  (sql)      postgres://demo:demo76950@127.0.0.1:26260?sslmode=require
  (sql/unix) postgres://demo:demo76950@?host=%2Fvar%2Ffolders%2Fc8%2Fb_q93vjj0ybfz0fz0z8vy9zc0000gp%2FT%2Fdemo070856957&port=26260

node 5:
  (webui)    http://127.0.0.1:8084/demologin?password=demo76950&username=demo
  (sql)      postgres://demo:demo76950@127.0.0.1:26261?sslmode=require
  (sql/unix) postgres://demo:demo76950@?host=%2Fvar%2Ffolders%2Fc8%2Fb_q93vjj0ybfz0fz0z8vy9zc0000gp%2FT%2Fdemo070856957&port=26261

node 6:
  (webui)    http://127.0.0.1:8085/demologin?password=demo76950&username=demo
  (sql)      postgres://demo:demo76950@127.0.0.1:26262?sslmode=require
  (sql/unix) postgres://demo:demo76950@?host=%2Fvar%2Ffolders%2Fc8%2Fb_q93vjj0ybfz0fz0z8vy9zc0000gp%2FT%2Fdemo070856957&port=26262

node 7:
  (webui)    http://127.0.0.1:8086/demologin?password=demo76950&username=demo
  (sql)      postgres://demo:demo76950@127.0.0.1:26263?sslmode=require
  (sql/unix) postgres://demo:demo76950@?host=%2Fvar%2Ffolders%2Fc8%2Fb_q93vjj0ybfz0fz0z8vy9zc0000gp%2FT%2Fdemo070856957&port=26263

node 8:
  (webui)    http://127.0.0.1:8087/demologin?password=demo76950&username=demo
  (sql)      postgres://demo:demo76950@127.0.0.1:26264?sslmode=require
  (sql/unix) postgres://demo:demo76950@?host=%2Fvar%2Ffolders%2Fc8%2Fb_q93vjj0ybfz0fz0z8vy9zc0000gp%2FT%2Fdemo070856957&port=26264

node 9:
  (webui)    http://127.0.0.1:8088/demologin?password=demo76950&username=demo
  (sql)      postgres://demo:demo76950@127.0.0.1:26265?sslmode=require
  (sql/unix) postgres://demo:demo76950@?host=%2Fvar%2Ffolders%2Fc8%2Fb_q93vjj0ybfz0fz0z8vy9zc0000gp%2FT%2Fdemo070856957&port=26265
```

You can shut down and restart any node by node id. For example, to shut down the 3rd node and then restart it:

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
> \demo shutdown 3
```

```
node 3 has been shutdown
```

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
> \demo restart 3
```

```
node 3 has been restarted
```

You can also decommission the 3rd node and then recommission it:

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
> \demo decommission 3
```

```
node 3 has been decommissioned
```

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
> \demo recommission 3
```

```
node 3 has been recommissioned
```

To add a new node to the cluster:

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
> \demo add region=us-central1,zone=a
```

```
node 10 has been added with locality "region=us-central1,zone=a"
```

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
> SHOW REGIONS FROM CLUSTER;
```

```
     region    |  zones
---------------+----------
  europe-west1 | {b,c,d}
  us-central1  | {a}
  us-east1     | {b,c,d}
  us-west1     | {a,b,c}
(4 rows)
```

```sql theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
> \demo ls
```

```
node 1:
  (webui)    http://127.0.0.1:8080/demologin?password=demo76950&username=demo
  (sql)      postgres://demo:demo76950@127.0.0.1:26257?sslmode=require
  (sql/unix) postgres://demo:demo76950@?host=%2Fvar%2Ffolders%2Fc8%2Fb_q93vjj0ybfz0fz0z8vy9zc0000gp%2FT%2Fdemo070856957&port=26257

node 2:
  (webui)    http://127.0.0.1:8081/demologin?password=demo76950&username=demo
  (sql)      postgres://demo:demo76950@127.0.0.1:26258?sslmode=require
  (sql/unix) postgres://demo:demo76950@?host=%2Fvar%2Ffolders%2Fc8%2Fb_q93vjj0ybfz0fz0z8vy9zc0000gp%2FT%2Fdemo070856957&port=26258

...

node 10:
  (webui)    http://127.0.0.1:8089/demologin?password=demo76950&username=demo
  (sql)      postgres://demo:demo76950@127.0.0.1:26266?sslmode=require
  (sql/unix) postgres://demo:demo76950@?host=%2Fvar%2Ffolders%2Fc8%2Fb_q93vjj0ybfz0fz0z8vy9zc0000gp%2FT%2Fdemo070856957&port=26266
```

### Try your own scenario

In addition to using one of the [pre-loaded dataset](#datasets), you can create your own database (e.g., <InternalLink path="create-database">`CREATE DATABASE <yourdb>;`</InternalLink>), or use the empty `defaultdb` database (e.g., <InternalLink path="set-vars">`SET DATABASE defaultdb;`</InternalLink>) to test our your own scenario involving any CockroachDB SQL features you are interested in.

## See also

* <InternalLink path="cockroach-sql#sql-flag-format">`cockroach sql`</InternalLink>
* <InternalLink path="cockroach-workload">`cockroach workload`</InternalLink>
* <InternalLink path="cockroach-commands">`cockroach` Commands Overview</InternalLink>
* <InternalLink path="sql-statements">SQL Statements</InternalLink>
* <InternalLink path="learn-cockroachdb-sql">Learn CockroachDB SQL</InternalLink>
* <InternalLink path="movr">MovR: Vehicle-Sharing App</InternalLink>

