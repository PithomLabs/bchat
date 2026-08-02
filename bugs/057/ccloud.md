> ## Documentation Index
> Fetch the complete documentation index at: https://www.cockroachlabs.com/llms.txt
> Use this file to discover all available pages before exploring further.

# ccloud CLI Command Reference

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

export const version = "stable";

This page describes how to create and manage CockroachDB Cloud clusters. If you have not installed the `ccloud` tool or logged into CockroachDB Cloud, refer to <InternalLink path="ccloud-get-started">Get Started with the `ccloud` CLI</InternalLink>.

## Log in to a CockroachDB Cloud organization

Run the `ccloud auth login` command and press **Enter** to open a browser window:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud auth login
```

If you are a member of more than one CockroachDB Cloud organization, use the `--org` flag to set the organization name when authenticating.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud auth login --org {organization-label}
```

## Create a new cluster

There are two ways to create clusters using `ccloud`: `ccloud quickstart create` and `ccloud cluster create`.

The `ccloud quickstart create` command interactively guides you through creating and connecting to a new CockroachDB Basic cluster.

The `ccloud cluster create` command creates a new CockroachDB Cloud cluster in your organization with the desired plan.

### Create a Basic cluster

Use the `ccloud cluster create basic` command to create a new CockroachDB Basic cluster.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster create basic
```

This command creates a CockroachDB Basic cluster in the default cloud infrastructure provider (GCP) and the closest region for that provider. It will generate a cluster name.

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Creating cluster...
Success! Created cluster
  name: blue-dog
  id: ec5e50eb-67dd-4d25-93b0-91ee7ece778d
```

The `id` in the output is the cluster ID. Use the `name` in other `ccloud` commands to identify the cluster on which the `ccloud` command operates.

You can set the cluster name, cloud infrastructure provider, region, and <InternalLink version="stable" path="architecture/glossary#resource-limits">resource limits</InternalLink> as command options. The following command is equivalent to the previous command that uses the default values.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster create basic blue-dog us-central1 --cloud GCP --spend-limit 0
```

### Create a Standard cluster

Use the `ccloud cluster create standard` command to create a new CockroachDB Standard cluster.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster create standard
```

This command creates a CockroachDB Standard cluster in the default cloud infrastructure provider (GCP) and the closest region for that provider. It will generate a cluster name.

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Creating cluster...
Success! Created cluster
  name: blue-dog
  id: ec5e50eb-67dd-4d25-93b0-91ee7ece778d
```

The `id` in the output is the cluster ID. Use the `name` in other `ccloud` commands to identify the cluster on which the `ccloud` command operates.

You can set the cluster name, cloud infrastructure provider, region, and <InternalLink version="stable" path="architecture/glossary#resource-limits">resource limits</InternalLink> as command options. The following command is equivalent to the previous command that uses the default values.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster create serverless blue-dog us-central1 --cloud GCP --spend-limit 0
```

### Create an Advanced cluster

Use the `ccloud cluster create dedicated` command to create a new CockroachDB Advanced cluster.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster create dedicated
```

This command creates a 1-node CockroachDB Advanced cluster with 4 virtual CPUs (vCPUs) and 110 GiB of storage in the default cloud infrastructure provider (GCP) and the closest region for that provider. It will generate a cluster name. The CockroachDB version will be the latest stable version.

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Creating cluster
Success! Created cluster
  name: blue-dog
  id: ec5e50eb-67dd-4d25-93b0-91ee7ece778d
```

The `id` in the output is the cluster ID. Use the `name` in other `ccloud` commands to identify the cluster on which the `ccloud` command operates.

You can set the cluster name, cloud infrastructure provider, region, number of nodes, and storage as command options. The following command is equivalent to the previous command that uses the default values.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster create dedicated blue-dog us-central1:1 --cloud GCP --vcpus 4 --storage-gib 110
```

When creating multi-region clusters, you must specify how many nodes should be in each region supported by the cloud infrastructure provider. For example, the following command creates a 12-node cluster where 8 nodes are in `us-central1` and 4 nodes are in `us-west2`. For optimum performance, it is generally recommended to configure the same number of nodes in each region.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster create dedicated blue-dog us-central1:8 us-west2:4 --cloud GCP --vcpus 4 --storage-gib 110
```

## List clusters in an organization

Use the `ccloud cluster list` command to show information about the clusters in your organization. It outputs columns with the cluster name, the cluster ID, the cluster plan, the creation date, the cluster's current state, the cloud provider, and the version of CockroachDB.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster list
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving clusters...
NAME      ID                                    PLAN TYPE        CREATED AT                            STATE                   CLOUD               VERSION
blue-dog   041d4c6b-69b9-4121-9c5a-8dd6ffd6b73d  PLAN_DEDICATED   2022-03-22 21:07:35.7177 +0000 UTC    CLUSTER_STATE_CREATING  CLOUD_PROVIDER_GCP  v21.2.4
...
```

## Get information about a cluster

Use the `ccloud cluster info` command with the cluster name as the parameter to show detailed information about your cluster. Find the **Name** column in the output of [`ccloud cluster list`](#list-clusters-in-an-organization) to find the name of the cluster.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster info blue-dog
```

Showing cluster info for a Basic or Standard cluster:

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving cluster...
Cluster info
 name: blue-dog
 id: 041d4c6b-69b9-4121-9c5a-8dd6ffd6b73d
 cockroach version: v25.4
 cloud: CLOUD_PROVIDER_GCP
 plan type: PLAN_SERVERLESS
 state: CLUSTER_STATE_CREATED
 resource limit: 0
 regions: us-central1
```

Showing cluster info for a CockroachDB Advanced cluster:

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving cluster...
Cluster info
 name: ievans-blue-dog-dos
 id: 041d4c6b-69b9-4121-9c5a-8dd6ffd6b73d
 cockroach version: v25.4
 cloud: CLOUD_PROVIDER_GCP
 plan type: PLAN_DEDICATED
 state: CLUSTER_STATE_CREATING
 hardware per node:
  4 vCPU
  7.500000 GiB RAM
  110 GiB disk
  450 IOPS
 region nodes:
  us-central1: 1
```

## Delete a cluster

Use the `ccloud cluster delete` command to delete the specified cluster using the cluster name.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster delete blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Deleting cluster...
Success! Deleted cluster
 id: 041d4c6b-69b9-4121-9c5a-8dd6ffd6b73d
```

<Note>
  If the cluster state is `CLUSTER_STATE_CREATING` you cannot delete the cluster. You must wait until the cluster has been provisioned and started, with a status of `CLUSTER_STATE_CREATED`, before you can delete the cluster. CockroachDB Serverless clusters are created in less than a minute. CockroachDB Advanced clusters can take an hour or more to provision and start.
</Note>

## Connect to a cluster with a SQL client

Use the `ccloud cluster sql` command to start a CockroachDB SQL shell connection to the specified cluster using the [cluster ID](#list-clusters-in-an-organization). If you haven't created a <InternalLink version="stable" path="security-reference/authorization#sql-users">SQL user</InternalLink> for the specified cluster, you will be prompted to create a new user and set the user password.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster sql blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving cluster info...
∙∙∙ Retrieving SQL user list...
No SQL users found. Create one now: y
Create a new SQL user:
Username: user
Password: ****************
∙∙∙ Creating SQL user...
Success! Created SQL user
 name: user
 cluster: 041d4c6b-69b9-4121-9c5a-8dd6ffd6b73d
Starting CockroachDB SQL shell...
#
# Welcome to the CockroachDB SQL shell.
# All statements must be terminated by a semicolon.
# To exit, type: \q.
#
# Client version: CockroachDB CCL v21.2.5 (x86_64-apple-darwin19, built 2022/02/07 21:04:05, go1.16.6)
# Server version: CockroachDB CCL v21.2.4-1-g70835279ac (x86_64-unknown-linux-gnu, built 2022/02/03 22:31:25, go1.16.6)
#
# Cluster ID: 041d4c6b-69b9-4121-9c5a-8dd6ffd6b73d
#
# Enter \? for a brief introduction.
#
```

### Connect to your cluster using SSO

Use the `--sso` flag to connect to your cluster using <InternalLink path="cloud-sso-sql">single sign-on (SSO) authentication</InternalLink>, which will allow you to start a SQL shell without using a password.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster sql --sso blue-dog
```

This will open a browser window on the local machine where you will log in to your organization if you are not already authenticated.

If you are running `ccloud` on a remote machine, use the `--no-redirect` flag. `ccloud` will output a URL that you must copy and paste in your local machine's browser in order to authenticate. After authentication, paste in the authorization code you received in the remote terminal to complete the login process.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster sql --sso --no-redirect blue-dog
```

Using SSO login requires that a separate SSO SQL user for your account is created on the cluster you are connecting to. SSO SQL usernames are prefixed with `sso_`. The SSO SQL username you use must match the SSO SQL username generated for you.

To create a SSO SQL user:

1. Connect to the cluster using the `--sso` flag.

   ```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
   ccloud cluster sql --sso blue-dog
   ```
2. Log in to your organization when prompted by `ccloud`.
3. Copy the command in the error message to create the SSO SQL user with the correct username.

   You must have `admin` privileges to create the SSO SQL user.
4. Create the SSO SQL user by pasting and running the command you copied.

   For example, if the command in the error message creates a `sso_maxroach` user:

   ```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
   ccloud cluster user create blue-dog sso_maxroach
   ```
5. Re-run the SQL client command to login and connect to your cluster.

   ```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
   ccloud cluster sql blue-dog --sso
   ```

Use the `ccloud auth whoami` command to check that you are logged into the correct organization.

If the organization is incorrect:

1. Log out of the current organization.

   ```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
   ccloud auth logout
   ```
2. Log in to the correct organization.

   ```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
   ccloud auth login --org {organization name}
   ```

### Skip the IP allowlist check when connecting to your cluster

By default, the `ccloud cluster sql` command will allow connections only from IP addresses in your cluster's <InternalLink path="network-authorization#ip-allowlisting">allowlist</InternalLink>. Use the `--skip-ip-check` flag to disable the client-side IP allowlist check:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster sql blue-dog --skip-ip-check
```

## Create a SQL user

Use the `ccloud cluster user create` command to create a new <InternalLink version="stable" path="security-reference/authorization#sql-users">SQL user</InternalLink> by passing in the cluster name and the username. By default, newly created users are assigned to the `admin` role. An `admin` SQL user has full privileges for all databases and tables in your cluster. This user can also create additional users and grant them appropriate privileges.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster user create blue-dog maxroach
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
Password: ****************
∙∙∙ Creating SQL user...
```

## Get connection information for a cluster

Use the `ccloud cluster sql` command to get connection information for the specified cluster using the cluster name.

To get the <InternalLink version="stable" path="connection-parameters#connect-using-a-url">connection URL</InternalLink>, use the `--connection-url` option.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster sql --connection-url blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving cluster info...
postgresql://blue-dog-5bct.gcp-us-east4.cockroachlabs.cloud:26257/defaultdb?sslmode=verify-full&sslrootcert=%2FUsers%2Fuser%2FLibrary%2FCockroachCloud%2Fcerts%2Fblue-dog-ca.crt
```

To get the individual connection parameters, use the `--connection-params` option.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster sql --connection-params blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving cluster info...
Connection parameters
 Database:  defaultdb
 Host:      blue-dog-5bct.gcp-us-east4.cockroachlabs.cloud
 Port:      26257
```

## Manage databases

The `ccloud cluster database` command lists, creates, and deletes databases within a cluster.

Use `ccloud cluster database list` to list all databases in a cluster:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster database list blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving databases...
NAME        TABLE COUNT
defaultdb   0
myapp       12
```

Use `ccloud cluster database create` to create a new database:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster database create blue-dog myapp
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Creating database...
Successfully created database 'myapp'
```

Use `ccloud cluster database delete` to delete a database:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster database delete blue-dog myapp
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Deleting database...
Successfully deleted database 'myapp'
```

## Manage cluster backups

The `ccloud cluster backup` commands lists backups and manages <InternalLink path="backup-and-restore-overview#cockroachdb-cloud-backups">backup configuration</InternalLink> for a cluster.

Use `ccloud cluster backup list` to list backups for a cluster:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster backup list blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving backups...
BACKUP ID                             AS OF TIME
a1b2c3d4-e5f6-7890-abcd-ef1234567890  2026-03-01 10:30:00Z
b2c3d4e5-f6a7-8901-bcde-f12345678901  2026-02-28 10:30:00Z
```

Use `ccloud backup config get` to retrieve the specified backup configuration:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster backup config get blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving backup configuration...
Cluster: blue-dog
Backups Enabled: Yes
Frequency: Every 60 minutes
Retention: 30 days
```

Use `ccloud cluster backup config update` to update the specified backup configuration:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster backup config update blue-dog --enabled true --frequency 120 --retention 60
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Updating backup configuration...
Success! Updated backup configuration
Cluster: blue-dog
Backups Enabled: Yes
Frequency: Every 120 minutes
Retention: 60 days
```

### Restore from a backup

The `ccloud cluster restore` commands list and create restore operations from backups.

Use `ccloud cluster restore list` to list restores for a cluster:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster restore list blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving restores...
ID                                    BACKUP ID                             TYPE     STATUS     COMPLETION %  CREATED AT
c3d4e5f6-a7b8-9012-cdef-123456789012  a1b2c3d4-e5f6-7890-abcd-ef1234567890  CLUSTER  SUCCESS    100%          2026-03-01 12:00:00Z
```

Use `ccloud cluster restore create` to start a restore from a specific backup to a destination cluster:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster restore create blue-dog --backup-id a1b2c3d4-e5f6-7890-abcd-ef1234567890
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Creating restore...
Successfully initiated restore
Restore ID: d4e5f6a7-b8c9-0123-defa-234567890123
Backup ID: a1b2c3d4-e5f6-7890-abcd-ef1234567890
Type: CLUSTER
Status: PENDING
```

If you are restoring a backup from a different cluster, specify the source cluster ID with the `--source-cluster-id` flag:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster restore create blue-dog --source-cluster-id a1b2c3d4-e5f6-7890-abcd-ef1234567890
```

You can also specify the restore type (`CLUSTER`, `DATABASE`, or `TABLE`) using the `--type` flag:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster restore create blue-dog --backup-id a1b2c3d4-e5f6-7890-abcd-ef1234567890 --type DATABASE
```

## List available CockroachDB versions

Use the `ccloud cluster versions` command to list the CockroachDB major versions available for new clusters or upgrades.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster versions
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving versions...
VERSION  RELEASE TYPE  SUPPORT STATUS  SUPPORT END  ALLOWED UPGRADES
v25.2    REGULAR       SUPPORTED       2026-11-18
v25.1    REGULAR       SUPPORTED       2026-05-19   v25.2
v24.3    REGULAR       SUPPORTED       2025-11-18   v25.1, v25.2
```

## Manage upgrade deferrals

The `ccloud cluster version-deferral` commands get or set the version upgrade deferral policy for a CockroachDB Advanced cluster. Version deferral lets you delay automatic major version upgrades.

Use `ccloud cluster version-deferral get` to retrieve the current deferral policy:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster version-deferral get blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving version deferral...
Deferral Policy: NOT_DEFERRED
```

Use `ccloud cluster version-deferral set` to set the deferral policy:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster version-deferral set blue-dog --policy DEFERRAL_60_DAYS
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Setting version deferral...
Successfully set version deferral
Deferral Policy: DEFERRAL_60_DAYS
```

Valid deferral policies are `NOT_DEFERRED`, `DEFERRAL_30_DAYS`, `DEFERRAL_60_DAYS`, and `DEFERRAL_90_DAYS`.

## Manage blackout windows

<Note>
  **This feature is in <InternalLink version="releases" path="cockroachdb-feature-availability">limited access</InternalLink>** and is only available to enrolled organizations. To enroll your organization, contact your Cockroach Labs account team. This feature is subject to change.
</Note>

The `ccloud cluster blackout-window` commands manage blackout windows for a CockroachDB Advanced cluster. Blackout windows prevent automatic maintenance operations during specified time periods.

Use `ccloud cluster blackout-window list` to retrieve blackout windows:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster blackout-window list blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving blackout windows...
ID                                    START TIME            END TIME
e5f6a7b8-c9d0-1234-efab-345678901234  2026-04-01 00:00:00Z  2026-04-07 00:00:00Z
```

To create a blackout window, specify the start and end times in RFC3339 format with the `--start` and `--end` flags. The start time must be at least 7 days in the future, and the end time must be within 14 days of the start time.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster blackout-window create blue-dog --start 2026-04-01T00:00:00Z --end 2026-04-07T00:00:00Z
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Creating blackout window...
Successfully created blackout window
ID: e5f6a7b8-c9d0-1234-efab-345678901234
Start: 2026-04-01 00:00:00Z
End: 2026-04-07 00:00:00Z
```

Use `ccloud cluster blackout-window delete` to delete a blackout window:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster blackout-window delete blue-dog e5f6a7b8-c9d0-1234-efab-345678901234
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Deleting blackout window...
Successfully deleted blackout window 'e5f6a7b8-c9d0-1234-efab-345678901234'
```

## Manage maintenance windows

The `ccloud cluster maintenance` commands configure the preferred maintenance window for a CockroachDB Advanced cluster. The maintenance window determines when automatic maintenance operations are performed. The window duration must be at least 6 hours and less than 1 week.

Use `ccloud cluster maintenance get` to retrieve the current maintenance window:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster maintenance get blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving maintenance window...
Cluster: blue-dog
Window Start: Tuesday 02:00 UTC
Window Duration: 6h
```

Use `ccloud cluster maintenance set` to set a maintenance window using the `--day`, `--hour`, and `--duration` flags:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster maintenance set blue-dog --day tuesday --hour 2 --duration 6h
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Setting maintenance window...
Success! Set maintenance window
Cluster: blue-dog
Window Start: Tuesday 02:00 UTC
Window Duration: 6h
```

Alternatively, you can specify the window start time as a raw offset from Monday 00:00 UTC using the `--offset` flag:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster maintenance set blue-dog --offset 26h --duration 6h
```

Use `ccloud cluster maintenance delete` to delete the maintenance window, resetting to default:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster maintenance delete blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Deleting maintenance window...
Success! Deleted maintenance window for cluster blue-dog
```

## Simulate cluster disruptions

<Note>
  **This feature is in <InternalLink version="releases" path="cockroachdb-feature-availability">limited access</InternalLink>** and is only available to enrolled organizations. To enroll your organization, contact your Cockroach Labs account team. This feature is subject to change.
</Note>

The `ccloud cluster disruption` commands simulate cluster disruptions for disaster recovery testing on a CockroachDB Advanced cluster. Disruptions allow you to test how your applications behave when parts of your cluster become unavailable.

Use `ccloud cluster disruption get` to retrieve the current disruption status:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster disruption get blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving disruption status...
No disruptions active
```

Use `ccloud cluster disruption set` to disrupt a whole region as specified with the `--region` and `--whole-region` flags:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster disruption set blue-dog --region us-east-1 --whole-region
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Setting disruption...
Successfully set disruption for region us-east-1
```

Use the `--azs` flag to disrupt specific availability zones within the specified region:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster disruption set blue-dog --region us-east-1 --azs us-east-1a,us-east-1b
```

Use `ccloud cluster disruption clear` to clear all disruptions and restore normal operation:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster disruption clear blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Clearing disruptions...
Successfully cleared all disruptions
```

## View CMEK configuration

Use the `ccloud cluster cmek get` command to view the <InternalLink path="cmek">Customer-Managed Encryption Keys (CMEK)</InternalLink> configuration for a CockroachDB Advanced cluster.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster cmek get blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving CMEK configuration...
Cluster: blue-dog
CMEK Status: ENABLED

Region Keys:
  Region: us-east-1
    Key URI: arn:aws:kms:us-east-1:123456789:key/a1b2c3d4-e5f6-7890-abcd-ef
    Status: ENABLED
```

## Configure log export

The `ccloud cluster log-export` commands configure log export for a CockroachDB Advanced cluster. You can export logs to AWS CloudWatch, GCP Cloud Logging, or Azure Log Analytics.

Use `ccloud cluster log-export get` to retrieve the current log export configuration:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster log-export get blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving log export configuration...
Cluster: blue-dog
Log Export Status: ENABLED
Type: AWS_CLOUDWATCH
Log Name: cockroach-logs
Auth Principal: arn:aws:iam::123456789:role/CockroachCloudLogExport
```

Use `ccloud cluster log-export enable` to enable log export. The following example configuration enables log export to AWS CloudWatch:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster log-export enable blue-dog --type AWS_CLOUDWATCH --auth-principal arn:aws:iam::123456789:role/CockroachCloudLogExport --log-name cockroach-logs
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Enabling log export...
Success! Enabled log export
Cluster: blue-dog
Type: AWS_CLOUDWATCH
Status: ENABLING
```

Use the `ccloud cluster log-export disable` command to disable log export:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster log-export disable blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Disabling log export...
Success! Disabled log export for cluster blue-dog
```

## Configure metric export

The `ccloud cluster metric-export` commands configure metric exporting for a CockroachDB Advanced cluster. You can export metrics to AWS CloudWatch, Datadog, or Prometheus as described in the following sections.

### Export metrics to AWS CloudWatch

Use `ccloud cluster metric-export cloudwatch enable` to enable metric export to AWS CloudWatch:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster metric-export cloudwatch enable blue-dog --role-arn arn:aws:iam::123456789:role/metrics-role --target-region us-east-1
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Enabling CloudWatch metric export...
Success! Enabled CloudWatch metric export
Cluster: blue-dog
Status: ENABLING
```

Use `ccloud cluster metric-export cloudwatch get` to retrieve the current CloudWatch metric export configuration:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster metric-export cloudwatch get blue-dog
```

Use `ccloud cluster metric-export cloudwatch disable` to disable CloudWatch metric export:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster metric-export cloudwatch disable blue-dog
```

### Export metrics to Datadog

Use `ccloud cluster metric-export datadob enable` to enable metric export to Datadog:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster metric-export datadog enable blue-dog --site US5 --api-key your-datadog-api-key
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Enabling Datadog metric export...
Success! Enabled Datadog metric export
Cluster: blue-dog
Site: US5
Status: ENABLING
```

Use `ccloud cluster metric-export datadog get` to retrieve the current Datadog metric export configuration:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster metric-export datadog get blue-dog
```

Use `ccloud cluster metric-export datadog disable` to disable Datadog metric export:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster metric-export datadog disable blue-dog
```

### Export metrics to Prometheus

Use `ccloud cluster metric-export prometheus enable` to enable metric export to Prometheus:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster metric-export prometheus enable blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Enabling Prometheus metric export...
Success! Enabled Prometheus metric export
Cluster: blue-dog
Status: ENABLING
```

Use `ccloud cluster metric-export prometheus get` to retrieve the Prometheus scrape endpoint:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster metric-export prometheus get blue-dog
```

Use `ccloud cluster metric-export prometheus disable` to disable the Prometheus endpoint:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster metric-export prometheus disable blue-dog
```

## Create and manage IP allowlists

The `ccloud cluster networking allowlist` commands create or manage an <InternalLink path="network-authorization#ip-allowlisting">IP allowlist</InternalLink>, which allows incoming network connections from the specified network IP range. Use the `--sql` flag to allow incoming CockroachDB SQL shell connections from the specified network. Use the `--ui` flag to allow access to the <InternalLink version="stable" path="ui-overview">DB Console</InternalLink> from the specified network.

The IP range must be in [Classless Inter-Domain Routing (CIDR) format](https://wikipedia.org/wiki/Classless_Inter-Domain_Routing). For more information on CIDR, see [Understanding IP Addresses, Subnets, and CIDR Notation for Networking](https://www.digitalocean.com/community/tutorials/understanding-ip-addresses-subnets-and-cidr-notation-for-networking#cidr-notation).

Use the `ccloud cluster networking allowlist create` command to create an allowlist. For example, to allow incoming connections from a single IP address, 1.1.1.1, to your cluster, including the CockroachDB SQL shell and DB Console, use the following command:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster networking allowlist create blue-dog 1.1.1.1/32 --sql --ui
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Creating IP allowlist entry...
Success! Created IP allowlist entry for
 network: 1.1.1.1/32
 cluster: 041d4c6b-69b9-4121-9c5a-8dd6ffd6b73d
```

Use the `ccloud cluster networking allowlist list` command to list the IP allowlists for your cluster:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster networking allowlist list blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving cluster allowlist...
NETWORK         NAME  UI  SQL
1.1.1.1/32            ✔   ✔
```

To modify an allowlist entry, use the `ccloud cluster networking allowlist update` command. The following command adds a descriptive name to the previously created entry:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster networking allowlist update blue-dog 1.1.1.1/32 --name home
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Updating IP allowlist entry...
Success! Updated IP allowlist entry for
 network: 1.1.1.1/32
 cluster: 041d4c6b-69b9-4121-9c5a-8dd6ffd6b73d
```

Rerunning the `allowlist list` command shows the updated entry:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster networking allowlist list blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving cluster allowlist...
NETWORK         NAME  UI  SQL
1.1.1.1/32      home  ✔   ✔
```

Use the `ccloud cluster networking allowlist delete` command to delete an entry:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster networking allowlist delete blue-dog 1.1.1.1/32
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Deleting IP allowlist entry...
Success! Deleted IP allowlist entry for
 network: 1.1.1.1/32
 cluster: 041d4c6b-69b9-4121-9c5a-8dd6ffd6b73d
```

## Manage egress networking rules

The `ccloud cluster networking egress-rule` commands manage egress traffic rules for a CockroachDB Advanced cluster. Egress rules control which external destinations your cluster can connect to.

Use `ccloud cluster networking egress-rule list` to retrieve egress rules:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster networking egress-rule list blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving egress rules...
ID                                    NAME           TYPE  DESTINATION        DESCRIPTION
f6a7b8c9-d0e1-2345-fab0-456789012345  allow-s3       FQDN  s3.amazonaws.com   Allow S3 access
a7b8c9d0-e1f2-3456-ab01-567890123456  allow-subnet   CIDR  10.0.0.0/8         Internal network
```

Use `ccloud cluster networking egress-rule create` to create an egress rule:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster networking egress-rule create blue-dog --name allow-s3 --type FQDN --destination s3.amazonaws.com --description "Allow S3 access"
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Creating egress rule...
Successfully created egress rule
ID: f6a7b8c9-d0e1-2345-fab0-456789012345
Name: allow-s3
Type: FQDN
Destination: s3.amazonaws.com
```

Use `ccloud cluster networking egress-rule delete` to delete an egress rule:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster networking egress-rule delete blue-dog f6a7b8c9-d0e1-2345-fab0-456789012345
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Deleting egress rule...
Successfully deleted egress rule 'f6a7b8c9-d0e1-2345-fab0-456789012345'
```

## Manage client CA certificates

The `ccloud cluster networking client-ca-cert` commands <InternalLink path="client-certs-advanced">manage client CA certificates</InternalLink> for a CockroachDB Advanced cluster. Client CA certificates allow clients to authenticate using TLS certificates signed by your own certificate authority.

Use `ccloud cluster networking client-ca-cert get` to retrieve the current client CA certificate:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster networking client-ca-cert get blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving client CA certificate...
Status: IS_SET
Certificate:
-----BEGIN CERTIFICATE-----
MIIBxTCCAWugAwIBAgIRAJ...
-----END CERTIFICATE-----
```

Use `ccloud cluster networking client-ca-cert set` to create a client CA certificate from a PEM-encoded file specified with the `--cert-file` flag:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster networking client-ca-cert set blue-dog --cert-file /path/to/ca.crt
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Setting client CA certificate...
Successfully set client CA certificate
Status: IS_SET
```

Use `ccloud cluster networking client-ca-cert update` to update the client CA certificate:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster networking client-ca-cert update blue-dog --cert-file /path/to/new-ca.crt
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Updating client CA certificate...
Successfully updated client CA certificate
Status: IS_SET
```

Use `ccloud cluster networking client-ca-cert delete` to delete the client CA certificate:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster networking client-ca-cert delete blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Deleting client CA certificate...
Successfully deleted client CA certificate
```

## Manage private endpoints

The `ccloud cluster networking private-endpoint` commands manage <InternalLink path="connect-to-an-advanced-cluster#establish-private-connectivity">private endpoint connectivity</InternalLink> for a CockroachDB Advanced cluster. Private endpoints provide private connectivity using AWS PrivateLink, GCP Private Service Connect, or Azure Private Link.

### Manage private endpoint services

Use `ccloud cluster networking private-endpoint service list` to retrieve available private endpoint services for a cluster:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster networking private-endpoint service list blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving private endpoint services...
REGION       SERVICE ID                                                     CLOUD  STATUS      AVAILABILITY ZONES
us-east-1    com.amazonaws.vpce.us-east-1.vpce-svc-0123456789abcdef         AWS    AVAILABLE   us-east-1a,us-east-1b,us-east-1c
```

Use `ccloud cluster networking private-endpoint service create` to create private endpoint services for all regions in a cluster:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster networking private-endpoint service create blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Creating private endpoint services...
Success! Created private endpoint services:

REGION       SERVICE ID                                                     CLOUD  STATUS
us-east-1    com.amazonaws.vpce.us-east-1.vpce-svc-0123456789abcdef         AWS    CREATING
```

### Manage private endpoint connections

Use `ccloud cluster networking private-endpoint connection list` to retrieve private endpoint connections:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster networking private-endpoint connection list blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving private endpoint connections...
ENDPOINT ID                  SERVICE ID                                                     REGION       CLOUD  STATUS
vpce-0123456789abcdef0       com.amazonaws.vpce.us-east-1.vpce-svc-0123456789abcdef         us-east-1    AWS    AVAILABLE
```

Use `ccloud cluster networking private-endpoint connection add` to add a connection using your cloud provider's private endpoint identifier:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster networking private-endpoint connection add blue-dog vpce-0123456789abcdef0
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Adding private endpoint connection...
Success! Added private endpoint connection
 Endpoint ID: vpce-0123456789abcdef0
 Service ID: com.amazonaws.vpce.us-east-1.vpce-svc-0123456789abcdef
 Status: PENDING
```

Use `ccloud cluster networking private-endpoint connection remove` to remove a connection:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster networking private-endpoint connection remove blue-dog vpce-0123456789abcdef0
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Removing private endpoint connection...
Success! Removed private endpoint connection vpce-0123456789abcdef0
```

### Manage trusted owners

Trusted owners control which cloud provider accounts are allowed to establish private endpoint connections to your cluster.

Use `ccloud cluster networking private-endpoint trusted-owner list` to retrieve trusted owners:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster networking private-endpoint trusted-owner list blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving trusted owners...
ID                                    EXTERNAL OWNER ID  TYPE
a1b2c3d4-e5f6-7890-abcd-ef1234567890  123456789012       AWS_ACCOUNT_ID
```

Use `ccloud cluster networking private-endpoint trusted-owner add` to add a trusted owner:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster networking private-endpoint trusted-owner add blue-dog 123456789012 --type AWS_ACCOUNT_ID
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Adding trusted owner...
Success! Added trusted owner
 ID: a1b2c3d4-e5f6-7890-abcd-ef1234567890
 External Owner ID: 123456789012
 Type: AWS_ACCOUNT_ID
```

Use `ccloud cluster networking private-endpoint trusted-owner remove` to remove a trusted owner:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster networking private-endpoint trusted-owner remove blue-dog a1b2c3d4-e5f6-7890-abcd-ef1234567890
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Removing trusted owner...
Success! Removed trusted owner a1b2c3d4-e5f6-7890-abcd-ef1234567890
```

## Manage egress private endpoints

<Note>
  **This feature is in <InternalLink version="releases" path="cockroachdb-feature-availability">limited access</InternalLink>** and is only available to enrolled organizations. To enroll your organization, contact your Cockroach Labs account team. This feature is subject to change.
</Note>

The `ccloud cluster networking egress-private-endpoint` commands manage <InternalLink path="egress-private-endpoints">egress private endpoint connections</InternalLink> from a CockroachDB Advanced cluster. Egress private endpoints allow your cluster to connect to external services using private network connectivity.

Use `ccloud cluster networking egress-private-endpoint list` to retrieve egress private endpoints:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster networking egress-private-endpoint list blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving egress private endpoints...
ID                                    REGION       STATE      TARGET SERVICE
b8c9d0e1-f2a3-4567-b012-678901234567  us-east-1    AVAILABLE  com.amazonaws.vpce.us-east-1.vpce-svc-012345abcdef
```

Use `ccloud cluster networking egress-private-endpoint get` to retrieve details about the specified egress private endpoint:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster networking egress-private-endpoint get blue-dog b8c9d0e1-f2a3-4567-b012-678901234567
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving egress private endpoint...
ID: b8c9d0e1-f2a3-4567-b012-678901234567
Region: us-east-1
State: AVAILABLE
Target Service Type: PRIVATE_SERVICE
Target Service Identifier: com.amazonaws.vpce.us-east-1.vpce-svc-012345abcdef
Endpoint Address: 10.0.1.5
Endpoint Connection ID: vpce-0abc123def456789
Domain Names: example.com
```

Use `ccloud cluster networking egress-private-endpoint create` to create an egress private endpoint:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster networking egress-private-endpoint create blue-dog --region us-east-1 --target-service-identifier com.amazonaws.vpce.us-east-1.vpce-svc-012345abcdef --target-service-type PRIVATE_SERVICE
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Creating egress private endpoint...
Successfully created egress private endpoint
ID: b8c9d0e1-f2a3-4567-b012-678901234567
Region: us-east-1
State: PENDING
```

Valid target service types for the `--target-service-type` flag are `PRIVATE_SERVICE`, `MSK_SASL_SCRAM`, `MSK_SASL_IAM`, and `MSK_TLS`.

Use `ccloud cluster networking egress-private-endpoint delete` to delete an egress private endpoint:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud cluster networking egress-private-endpoint delete blue-dog b8c9d0e1-f2a3-4567-b012-678901234567
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Deleting egress private endpoint...
Successfully deleted egress private endpoint 'b8c9d0e1-f2a3-4567-b012-678901234567'
```

## View organization information

Use the `ccloud organization get` command (or its alias `ccloud org get`) to view information about your CockroachDB Cloud organization.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud organization get
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving organization...
ID: a1b2c3d4-e5f6-7890-abcd-ef1234567890
Name: my-organization
Label: my-org
Created At: 2024-01-15 10:30:00Z
```

## View audit logs

Use the `ccloud audit list` command to view audit log entries for your organization. Audit logs record actions taken on your CockroachDB Cloud resources, including who performed the action, when, and what was changed.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud audit list
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving audit logs...
TIME                  ACTION                CLUSTER     USER
2026-03-01 12:00:00Z  CLUSTER_CREATED       blue-dog    user@example.com
2026-03-01 11:30:00Z  SQL_USER_CREATED      blue-dog    user@example.com
2026-03-01 10:00:00Z  ALLOWLIST_CREATED     blue-dog    user@example.com
```

Use the `--limit` flag to control the number of entries returned, and `--starting-from` to filter by start time:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud audit list --limit 10 --starting-from 2026-03-01T00:00:00Z
```

## Manage billing

The `ccloud billing invoice` commands display invoices and billing information for your organization.

Use `ccloud billing invoice list` to retrieve a list of invoices:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud billing invoice list
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving invoices...
INVOICE ID                            PERIOD START          PERIOD END            STATUS     TOTAL
d0e1f2a3-b4c5-6789-0123-456789abcdef  2026-02-01 00:00:00Z  2026-02-28 23:59:59Z  FINALIZED  1,234.56 USD
e1f2a3b4-c5d6-7890-1234-567890abcdef  2026-01-01 00:00:00Z  2026-01-31 23:59:59Z  FINALIZED  1,100.00 USD
```

Use `ccloud billing invoice get` to retrieve the details of a specific invoice:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud billing invoice get d0e1f2a3-b4c5-6789-0123-456789abcdef
```

## Manage folders

The `ccloud folder` commands manage folders for organizing clusters within your organization.

Use `ccloud folder list` to retrieve a list of folders:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud folder list
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving folders...
ID                                    NAME          PARENT PATH           TYPE
f2a3b4c5-d6e7-8901-2345-678901abcdef  Production                          FOLDER
a3b4c5d6-e7f8-9012-3456-789012abcdef  Staging       /Production/Staging/  FOLDER
```

Use `ccloud folder get` to retrieve the details of a specific folder:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud folder get f2a3b4c5-d6e7-8901-2345-678901abcdef
```

Use `ccloud folder create` to create a folder:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud folder create Production
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Creating folder...
Success! Created folder
 ID: f2a3b4c5-d6e7-8901-2345-678901abcdef
 Name: Production
```

Use the `--parent-id` flag with a folder ID to create a subfolder:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud folder create Staging --parent-id f2a3b4c5-d6e7-8901-2345-678901abcdef
```

Use `ccloud folder update` to update a folder name:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud folder update f2a3b4c5-d6e7-8901-2345-678901abcdef --name "Prod Environment"
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Updating folder...
Success! Updated folder
 ID: f2a3b4c5-d6e7-8901-2345-678901abcdef
 Name: Prod Environment
```

Use `ccloud folder delete` to delete a folder:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud folder delete f2a3b4c5-d6e7-8901-2345-678901abcdef
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Deleting folder...
Success! Deleted folder f2a3b4c5-d6e7-8901-2345-678901abcdef
```

Use `ccloud folder contents` to retrieve the contents of a folder:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud folder contents f2a3b4c5-d6e7-8901-2345-678901abcdef
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving folder contents...
ID                                    NAME        TYPE     PARENT PATH
041d4c6b-69b9-4121-9c5a-8dd6ffd6b73d  my-cluster  CLUSTER  /Production
a3b4c5d6-e7f8-9012-3456-789012abcdef  Staging     FOLDER   /Production/Staging/
```

## Manage service accounts

The `ccloud service-account` commands manage service accounts for programmatic access to CockroachDB Cloud.

Use `ccloud service-account list` to retrieve a list of service accounts:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud service-account list
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving service accounts...
ID                                    NAME          DESCRIPTION        CREATED AT
b4c5d6e7-f8a9-0123-4567-890123abcdef  ci-pipeline   CI/CD automation   2026-01-15 10:30:00Z
```

Use `ccloud service-account get` to retrieve details for a specific service account:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud service-account get b4c5d6e7-f8a9-0123-4567-890123abcdef
```

Use `ccloud service-account create` to create a service account with a description specified by the `--description` flag:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud service-account create ci-pipeline --description "CI/CD automation"
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Creating service account...
Success! Created service account
 ID: b4c5d6e7-f8a9-0123-4567-890123abcdef
 Name: ci-pipeline
```

Use `ccloud service-account delete` to delete a service account:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud service-account delete b4c5d6e7-f8a9-0123-4567-890123abcdef
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Deleting service account...
Success! Deleted service account b4c5d6e7-f8a9-0123-4567-890123abcdef
```

### Manage API keys for service accounts

The `ccloud service-account api-key` commands manage API keys for a service account.

Use `ccloud service-account api-key list` to list API keys for the service account ID specified by the `--service-account-id` flag:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud service-account api-key list --service-account-id b4c5d6e7-f8a9-0123-4567-890123abcdef
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving API keys...
ID                                    NAME          SERVICE ACCOUNT ID                    CREATED AT
c5d6e7f8-a9b0-1234-5678-901234abcdef  deploy-key    b4c5d6e7-f8a9-0123-4567-890123abcdef  2026-01-15 10:30:00Z
```

Use `ccloud service-account api-key create` to create an API key:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud service-account api-key create b4c5d6e7-f8a9-0123-4567-890123abcdef deploy-key
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Creating API key...
Success! Created API key
 ID: c5d6e7f8-a9b0-1234-5678-901234abcdef
 Name: deploy-key
 Secret: CCDB1_ABCDEFghijklmnopqrstuvwxyz0123456789...

IMPORTANT: Save this secret now. It will not be shown again.
```

Use `ccloud service-account api-key delete` to delete an API key:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud service-account api-key delete c5d6e7f8-a9b0-1234-5678-901234abcdef
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Deleting API key...
Success! Deleted API key c5d6e7f8-a9b0-1234-5678-901234abcdef
```

## Manage JWT issuers

<Note>
  **This feature is in <InternalLink version="releases" path="cockroachdb-feature-availability">limited access</InternalLink>** and is only available to enrolled organizations. To enroll your organization, contact your Cockroach Labs account team. This feature is subject to change.
</Note>

The `ccloud jwt-issuer` commands manage JWT/OIDC identity providers for cluster authentication. JWT issuers allow your clusters to authenticate users via external identity providers.

Use `ccloud jwt-issuer list` to retrieve a list of JWT issuers:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud jwt-issuer list
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving JWT issuers...
ID                                    ISSUER URL                              AUDIENCE
d6e7f8a9-b0c1-2345-6789-012345abcdef  https://accounts.google.com             my-app
e7f8a9b0-c1d2-3456-7890-123456abcdef  https://login.microsoftonline.com/...   my-app-azure
```

Use `ccloud jwt-issuer get` to retrieve the details of a JWT issuer:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud jwt-issuer get d6e7f8a9-b0c1-2345-6789-012345abcdef
```

Use `ccloud jwt-issuer create` to create a JWT issuer with the specified `--issuer-url`, `--audience`, and `--claim` flag values:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud jwt-issuer create --issuer-url https://accounts.google.com --audience my-app --claim email
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Creating JWT issuer...
Successfully created JWT issuer
ID: d6e7f8a9-b0c1-2345-6789-012345abcdef
Issuer URL: https://accounts.google.com
Audience: my-app
```

Use `ccloud jwt-issuer update` to update a JWT issuer with the values of the specified flags:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud jwt-issuer update d6e7f8a9-b0c1-2345-6789-012345abcdef --audience my-app-v2
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Updating JWT issuer...
Successfully updated JWT issuer
ID: d6e7f8a9-b0c1-2345-6789-012345abcdef
Issuer URL: https://accounts.google.com
Audience: my-app-v2
```

Use `ccloud jwt-issuer delete` to delete a JWT issuer:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud jwt-issuer delete d6e7f8a9-b0c1-2345-6789-012345abcdef
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Deleting JWT issuer...
Successfully deleted JWT issuer 'd6e7f8a9-b0c1-2345-6789-012345abcdef'
```

## Manage physical cluster replication

<Note>
  **This feature is in <InternalLink version="releases" path="cockroachdb-feature-availability">limited access</InternalLink>** and is only available to enrolled organizations. To enroll your organization, contact your Cockroach Labs account team. This feature is subject to change.
</Note>

The `ccloud replication` commands manage <InternalLink version="stable" path="physical-cluster-replication-overview">physical cluster replication (PCR)</InternalLink> between CockroachDB Cloud clusters.

Use `ccloud replication list` to retrieve a list of replication streams for a cluster:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud replication list blue-dog
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving replication streams...
ID                                    PRIMARY CLUSTER                       STANDBY CLUSTER                       STATUS
f8a9b0c1-d2e3-4567-8901-234567abcdef  a1b2c3d4-e5f6-7890-abcd-ef1234567890  b2c3d4e5-f6a7-8901-bcde-f12345678901  REPLICATING
```

Use `ccloud replication get` to retrieve the details of a replication stream:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud replication get f8a9b0c1-d2e3-4567-8901-234567abcdef
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Retrieving replication stream...
ID: f8a9b0c1-d2e3-4567-8901-234567abcdef
Primary Cluster: a1b2c3d4-e5f6-7890-abcd-ef1234567890
Standby Cluster: b2c3d4e5-f6a7-8901-bcde-f12345678901
Status: REPLICATING
Created At: 2026-02-15 10:30:00Z
Replicated Time: 2026-03-04 12:00:00Z
Replication Lag: 5 seconds
```

Use `ccloud replication create` to create a replication stream between the clusters specified in the `--primary-cluster` and `--standby-cluster` flags:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud replication create --primary-cluster blue-dog --standby-cluster red-cat
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Creating replication stream...
Successfully created replication stream
ID: f8a9b0c1-d2e3-4567-8901-234567abcdef
Primary Cluster: a1b2c3d4-e5f6-7890-abcd-ef1234567890
Standby Cluster: b2c3d4e5-f6a7-8901-bcde-f12345678901
Status: STARTING
```

Use `ccloud replication update` with the `--status FAILING_OVER` flag to initiate a failover to the standby cluster:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud replication update f8a9b0c1-d2e3-4567-8901-234567abcdef --status FAILING_OVER
```

```text theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
∙∙∙ Updating replication stream...
Successfully updated replication stream
ID: f8a9b0c1-d2e3-4567-8901-234567abcdef
Status: FAILING_OVER
```

Use the `--failover-at` flag to schedule a failover for a specific time:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud replication update f8a9b0c1-d2e3-4567-8901-234567abcdef --status FAILING_OVER --failover-at 2026-03-05T00:00:00Z
```

Use `ccloud replication update` with the `--status CANCELED` flag to cancel a replication stream:

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud replication update f8a9b0c1-d2e3-4567-8901-234567abcdef --status CANCELED
```

## Turn off telemetry events for `ccloud`

Cockroach Labs collects anonymized telemetry events to improve the usability of `ccloud`. Use the `ccloud settings set` command and disable sending telemetry events to Cockroach Labs.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
ccloud settings set --disable-telemetry=true
```
