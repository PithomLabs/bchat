> ## Documentation Index
> Fetch the complete documentation index at: https://www.cockroachlabs.com/llms.txt
> Use this file to discover all available pages before exploring further.

# Production Checklist

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

This page provides important recommendations for production deployments of CockroachDB.

## Topology

When planning your deployment, it's important to carefully review and choose the <InternalLink path="topology-patterns">topology patterns</InternalLink> that best meet your latency and resiliency requirements. This is especially crucial for multi-region deployments.

Also keep in mind some basic topology recommendations:

* Do not run multiple node processes on the same VM or machine. This defeats CockroachDB's replication and causes the system to be a single point of failure. Instead, start each node on a separate VM or machine.
* To start a node with multiple disks or SSDs, provide a separate `--store` flag for each disk when starting the `cockroach` process on the node. For more details about stores, see <InternalLink path="cockroach-start#store">Start a Node</InternalLink>.

<Danger>
  If you start a node with multiple `--store` flags, it is not possible to scale back down to only using a single store on the node. Instead, you must decommission the node and start a new node with the updated `--store`.
</Danger>

* When starting each node, use the <InternalLink path="cockroach-start#locality">`--locality`</InternalLink> flag to describe the node's location, for example, `--locality=region=west,zone=us-west-1`. The key-value pairs should be ordered from most to least inclusive, and the keys and order of key-value pairs must be the same on all nodes.

* When deploying in a single availability zone:

  * To be able to tolerate the failure of any 1 node, use at least 3 nodes with the <InternalLink path="configure-replication-zones#view-the-default-replication-zone">`default` 3-way replication factor</InternalLink>. In this case, if 1 node fails, each range retains 2 of its 3 replicas, a majority.

  * To be able to tolerate 2 simultaneous node failures, use at least 5 nodes and <InternalLink path="configure-replication-zones#edit-the-default-replication-zone">increase the `default` replication factor for user data</InternalLink> to 5. The replication factor for <InternalLink path="configure-replication-zones#create-a-replication-zone-for-a-system-range">important internal data</InternalLink> is 5 by default, so no adjustments are needed for internal data. In this case, if 2 nodes fail at the same time, each range retains 3 of its 5 replicas, a majority.

* When deploying across multiple availability zones:

  * To be able to tolerate the failure of 1 entire AZ in a region, use at least 3 AZs per region and set `--locality` on each node to spread data evenly across regions and AZs. In this case, if 1 AZ goes offline, the 2 remaining AZs retain a majority of replicas.

  * To ensure that ranges are split evenly across nodes, use the same number of nodes in each AZ. This is to avoid overloading any nodes with excessive resource consumption.

* When deploying across multiple regions:

  * To be able to tolerate the failure of 1 entire region, use at least 3 regions.

For optimal cluster performance, Cockroach Labs recommends that all nodes use the same hardware and operating system.

## Software

We recommend running a [glibc](https://www.gnu.org/software/libc/)-based Linux distribution and Linux kernel version from the last 5 years, such as [Ubuntu](https://ubuntu.com/), [Red Hat Enterprise Linux (RHEL)](https://www.redhat.com/technologies/linux-platforms/enterprise-linux), [CentOS](https://www.centos.org/), or [Container-Optimized OS](https://cloud.google.com/container-optimized-os/docs).

We have observed increased memory usage in rare cases due to [transparent huge pages (THP)](https://www.kernel.org/doc/html/latest/admin-guide/mm/transhuge.html) being enabled (i.e., set to `always`). New deployments should configure THP with the `madvise` option.

Existing deployments that have THP enabled using the `always` option should change it to `madvise` unless they are currently running with a comfortable memory usage margin.

The method for permanently changing the THP setting across reboots depends on the operating system. For Red Hat Enterprise Linux, refer to the [Red Hat documentation](https://access.redhat.com/solutions/46111).

## Hardware

<Note>
  In our sizing and production guidance, 1 vCPU is considered equivalent to 1 core in the underlying hardware platform.
</Note>

### Sizing

The size of your cluster corresponds to its total number of vCPUs. This number depends holistically on your application requirements: total storage capacity, SQL workload response time, SQL [workload concurrency](#connection-pooling), and database service availability.

Working from your total vCPU count, you should then decide how many vCPUs to allocate to each machine. The larger the nodes (i.e., the more vCPUs on the machine), the fewer nodes will be in your cluster.

Carefully consider the following tradeoffs:

* A **smaller number of larger nodes** emphasizes cluster stability.

  * Larger nodes tolerate <InternalLink path="understand-hotspots">hotspots</InternalLink> more effectively than smaller nodes.
  * Queries operating on large data sets may strain network transfers if the data is spread widely over many smaller nodes. Having fewer and larger nodes enables more predictable workload performance.
  * A cluster with fewer nodes may be easier to operate and maintain.

* A **larger number of smaller nodes** emphasizes resiliency across <InternalLink path="disaster-recovery-planning">failure scenarios</InternalLink>.

  * The loss of a small node during failure or routine maintenance has a lesser impact on workload response time and concurrency.
  * Having more and smaller nodes allows <InternalLink path="take-and-restore-encrypted-backups">backup and restore jobs</InternalLink> to complete more quickly, since these jobs run in parallel and less data is hosted on each individual node.
  * Recovery from a failed node is faster when data is spread across more nodes. A smaller node will also take a shorter time to rebalance to a steady state.

In general, distribute your total vCPUs into the **largest possible nodes and smallest possible cluster** that meets your fault tolerance goals.

* For cluster stability, Cockroach Labs recommends a *minimum* of **8 vCPUs**, and strongly recommends no fewer than **4 vCPUs** per node. In a cluster with too few CPU resources, foreground client workloads will compete with the cluster's background maintenance tasks. For more information, see <InternalLink path="cluster-setup-troubleshooting#capacity-planning-issues">capacity planning issues</InternalLink>.

<Note>
  Clusters deployed in CockroachDB Cloud can be created with a minimum of 2 vCPUs per node on AWS and GCP or 4 vCPUs per node on Azure.
</Note>

* Avoid "burstable" or "shared-core" virtual machines that limit the load on CPU resources.

* Cockroach Labs does not extensively test clusters with more than **64 vCPUs** per node. This is the recommended *maximum* threshold.

* CockroachDB should only run on single-NUMA instances. Running a CockroachDB process across NUMA nodes is not recommended and can heavily impact performance. For more information on how CockroachDB interfaces with NUMA, read the <InternalLink path="install-cockroachdb-linux#numa">Linux deployment limitations</InternalLink>.

### Basic hardware recommendations

After you [size your cluster](#sizing), you can determine the amount of RAM, storage capacity, and disk I/O from the number of vCPUs.

This hardware guidance is meant to be platform agnostic and can apply to bare-metal, containerized, and orchestrated deployments. Also see our [cloud-specific](#cloud-specific-recommendations) recommendations.

| Value             | Recommendation | Reference             |
| ----------------- | -------------- | --------------------- |
| RAM per vCPU      | 4 GiB          | [Memory](#memory)     |
| Capacity per vCPU |                | [Storage](#storage)   |
| IOPS per vCPU     | 500            | [Disk I/O](#disk-i/o) |
| MB/s per vCPU     | 30             | [Disk I/O](#disk-i/o) |

Before deploying to production, test and tune your hardware setup for your application workload. For example, read-heavy and write-heavy workloads will place different emphases on [CPU](#sizing), [RAM](#memory), [storage](#storage), [I/O](#disk-i/o), and [network](#networking) capacity.

#### Memory

Provision at least **4 GiB of RAM per vCPU** for consistency across a variety of workload complexities. The minimum acceptable ratio is 2 GiB of RAM per vCPU, which is only suitable for testing.

<Note>
  The benefits to having more RAM decrease as the [number of vCPUs](#sizing) increases.
</Note>

* To optimize for the support of large numbers of tables, increase the amount of RAM. For more information, see <InternalLink path="schema-design-overview#quantity-of-tables-and-other-schema-objects">Quantity of tables and other schema objects</InternalLink>. Supporting a large number of rows is a matter of [Storage](#storage).

* To ensure consistent SQL performance, make sure all nodes have a uniform configuration.

* Disable Linux memory swapping. Over-allocating memory on production machines can lead to unexpected performance issues when pages have to be read back into memory.

* To help guard against <InternalLink path="cluster-setup-troubleshooting#out-of-memory-oom-crash">out-of-memory (OOM) crashes</InternalLink>, consider tuning the cache and SQL memory for cluster nodes. Refer to the section [Cache and SQL memory size](#cache-and-sql-memory-size).

* Monitor <InternalLink path="common-issues-to-monitor#cpu-usage">CPU</InternalLink> and <InternalLink path="common-issues-to-monitor#database-memory-usage">memory</InternalLink> usage. Ensure that they remain within acceptable limits.

<Note>
  Under-provisioning RAM results in reduced performance (due to reduced caching and increased spilling to disk), and in some cases can cause <InternalLink path="cluster-setup-troubleshooting#out-of-memory-oom-crash">OOM crashes</InternalLink>. For more information, see <InternalLink path="cluster-setup-troubleshooting#memory-issues">memory issues</InternalLink>.
</Note>

<a id="storage" />

#### Storage

We recommend provisioning volumes with <b>320 GiB per vCPU</b>. It's fine to have less storage per vCPU if your workload does not have significant capacity needs.

* The maximum recommended storage capacity per node is 10 TiB, regardless of the number of vCPUs.

* Use dedicated volumes for the CockroachDB store. Do not share the store volume with any other I/O activity.

* Determine where CockroachDB log files will be stored: either on the same volume as the main data store or on a separate volume from the main data store. Refer to Storage considerations for file sinks in Logging Best Practices.

* The recommended Linux filesystems are [ext4](https://ext4.wiki.kernel.org/index.php/Main_Page) and [XFS](https://xfs.wiki.kernel.org/).

* Always keep some of your disk capacity free on production. Doing so accommodates fluctuations in routine database operations and supports continuous data growth.

* <InternalLink path="common-issues-to-monitor#storage-capacity">Monitor your storage utilization</InternalLink> and rate of growth, and take action to add capacity well before you hit the limit.

* CockroachDB will <InternalLink path="cluster-setup-troubleshooting#automatic-ballast-files">automatically provision an emergency ballast file</InternalLink> at <InternalLink path="cockroach-start#flags-max-offset">node startup</InternalLink>. In the unlikely case that a node runs out of disk space and shuts down, you can delete the ballast file to free up enough space to be able to restart the node.

* Use <InternalLink path="configure-replication-zones">zone configs</InternalLink> to increase the replication factor from 3 (the default) to 5 (across at least 5 nodes).

  This is especially recommended if you are using local disks rather than a cloud provider's network-attached disks that are often replicated under the hood, because local disks have a greater risk of failure. You can do this for the <InternalLink path="configure-replication-zones#edit-the-default-replication-zone">entire cluster</InternalLink> or for specific <InternalLink path="configure-replication-zones#create-a-replication-zone-for-a-database">databases</InternalLink>, <InternalLink path="configure-replication-zones#create-a-replication-zone-for-a-table">tables</InternalLink>, or <InternalLink path="configure-replication-zones#create-a-replication-zone-for-a-partition">rows</InternalLink>.

* Considerations for on-premises storage infrastructure:
  * **Distributed file systems** such as CephFS and GlusterFS should not be used for CockroachDB store volumes. CockroachDB already handles <InternalLink path="architecture/distribution-layer">data distribution</InternalLink>, <InternalLink path="architecture/replication-layer">replication</InternalLink>, and fault tolerance using <InternalLink path="architecture/replication-layer#raft">Raft</InternalLink>. Adding a distributed file system underneath creates a second, uncoordinated layer that can cause duplicate replication, higher and more variable latency, and more complex failures.
  * **NAS or other storage that presents a filesystem interface over the network** should also not be used for CockroachDB store volumes. Examples include NFS, SMB/CIFS, and Amazon EFS mounted as a filesystem.
  * **SAN-backed storage** can be used when it presents a block device to the operating system that is formatted with a recommended Linux filesystem such as `ext4` or `xfs`. In that case, it is closer to cloud network-attached block storage than to a distributed file system. Be aware that the additional layers in I/O could break `fsync()` fidelity and lead to undesired behavior.

<Note>
  Under-provisioning storage leads to node crashes when the disks fill up. Once this has happened, it is difficult to recover from. To prevent your disks from filling up, provision enough storage for your workload, monitor your disk usage, and use a <InternalLink path="cluster-setup-troubleshooting#automatic-ballast-files">ballast file</InternalLink>. For more information, see <InternalLink path="cluster-setup-troubleshooting#capacity-planning-issues">capacity planning issues</InternalLink> and <InternalLink path="cluster-setup-troubleshooting#storage-issues">storage issues</InternalLink>.
</Note>

For instructions on how to free up disk space as quickly as possible after dropping a table, see <InternalLink path="operational-faqs#how-can-i-free-up-disk-space-when-dropping-a-table">How can I free up disk space that was used by a dropped table?</InternalLink>

<a id="disk-i/o" />

##### Disk I/O

Disks must be able to achieve <b>500 IOPS and 30 MB/s per vCPU</b>.

* <InternalLink path="common-issues-to-monitor#disk-iops">Monitor IOPS</InternalLink> using the DB Console and `iostat`. Ensure that they remain within acceptable values.

* Use [sysbench](https://github.com/akopytov/sysbench) to benchmark IOPS on your cluster. If IOPS decrease, add more nodes to your cluster to increase IOPS.

* Do not use LVM in the I/O path. Dynamically resizing CockroachDB store volumes can result in significant performance degradation. Using LVM snapshots in lieu of CockroachDB backup and restore is also not supported. Use multiple stores per node instead.

<Note>
  Disk I/O especially affects <InternalLink path="architecture/reads-and-writes-overview">performance on write-heavy workloads</InternalLink>. For more information, see <InternalLink path="cluster-setup-troubleshooting#capacity-planning-issues">capacity planning issues</InternalLink>.
</Note>

<a id="node-density-testing-configuration" />

##### Node density testing configuration

In a narrowly-scoped test, we were able to successfully store 4.32 TiB of logical data per node. The results of this test may not be applicable to your specific situation; testing with your workload is *strongly* recommended before using it in a production environment.

These results were achieved using the <InternalLink path="cockroach-workload#bank-workload">"bank" workload</InternalLink> running on AWS using 6x c5d.4xlarge nodes, each with 5 TiB of gp2 EBS storage.

Results:

| Value                 | Result           |
| --------------------- | ---------------- |
| vCPU per Node         | 16               |
| Logical Data per Node | 4.32 TiB         |
| RAM per Node          | 32 GiB           |
| Data per Core         | \~270 GiB / vCPU |
| Data per RAM          | \~135 GiB / GiB  |

### Cloud-specific recommendations

Based on our internal testing, we recommend the following cloud-specific configurations. Before using configurations not recommended here, be sure to test them exhaustively. Also consider the following workload-specific observations:

* For OLTP applications, small instance types may outperform larger instance types.
* Larger, more complex workloads will likely see more consistent performance from instance types with more available memory.
* Unless your workload requires extremely high IOPS or very low storage latency, the most cost-effective volumes are general-purpose rather than high-performance volumes.
  * Because storage cost influences the cost of running a workload much more than instance cost, larger nodes offer a better price-for-performance ratio at the same workload complexity.

#### AWS

* Use general-purpose [`m6i` or `m6a`](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/general-purpose-instances.html) VMs with SSD-backed [EBS volumes](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ebs-volume-types.html). For example, Cockroach Labs has used `m6i.2xlarge` for performance benchmarking. If your workload requires high throughput, use network-optimized `m5n` instances. To simulate bare-metal deployments, use `m5d` with [SSD Instance Store volumes](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ssd-instance-store.html).

  * `m5` and `m5a` instances, and [compute-optimized `c5`, `c5a`, and `c5n`](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/compute-optimized-instances.html) instances, are also acceptable.

<Danger>
  **Do not** use [burstable performance instances](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/burstable-performance-instances.html), which limit the load on a single core.
</Danger>

* [General Purpose SSD `gp3` volumes](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ebs-volume-types.html#gp3-ebs-volume-type) are a cost-effective storage option. `gp3` volumes provide 3,000 IOPS and 125 MiB/s throughput by default. If your deployment requires more IOPS or throughput, per our [hardware recommendations](#disk-i/o), you must provision these separately. [Provisioned IOPS SSD-backed (`io2`) EBS volumes](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ebs-volume-types.html#EBSVolumeTypes_piops) also need to have IOPS provisioned.

* A typical deployment uses [EC2](https://aws.amazon.com/ec2/) together with [key pairs](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-key-pairs.html), [load balancers](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/load-balancer-types.html), and [security groups](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/working-with-security-groups.html). For an example, refer to <InternalLink path="deploy-cockroachdb-on-aws">Deploy CockroachDB on AWS EC2</InternalLink>.

#### Azure

* Use general-purpose [Dsv5-series](https://docs.microsoft.com/azure/virtual-machines/dv5-dsv5-series) and [Dasv5-series](https://docs.microsoft.com/azure/virtual-machines/dasv5-dadsv5-series) or memory-optimized [Ev5-series](https://docs.microsoft.com/azure/virtual-machines/ev5-esv5-series) and [Easv5-series](https://docs.microsoft.com/azure/virtual-machines/easv5-eadsv5-series#easv5-series) VMs. For example, Cockroach Labs has used `Standard_D8s_v5`, `Standard_D8as_v5`, `Standard_E8s_v5`, and `Standard_e8as_v5` for performance benchmarking.

  * Compute-optimized [F-series](https://docs.microsoft.com/azure/virtual-machines/fsv2-series) VMs are also acceptable.

<Danger>
  Do not use ["burstable" B-series](https://docs.microsoft.com/azure/virtual-machines/linux/b-series-burstable) VMs, which limit the load on CPU resources. Also, Cockroach Labs has experienced data corruption issues on A-series VMs, so we recommend avoiding those as well.
</Danger>

* Use [Premium Storage](https://docs.microsoft.com/azure/virtual-machines/disks-types#premium-ssds) or local SSD storage with a Linux filesystem such as `ext4` (not the Windows `ntfs` filesystem). Note that [the size of a Premium Storage disk affects its IOPS](https://docs.microsoft.com/azure/virtual-machines/premium-storage-performance#iops).

* If you choose local SSD storage, on reboot, the VM can come back with the `ntfs` filesystem. Be sure your automation monitors for this and reformats the disk to the Linux filesystem you chose initially.

#### Digital Ocean

* Use any [droplets](https://www.digitalocean.com/pricing/) except standard droplets with only 1 GiB of RAM, which is below our minimum requirement. All Digital Ocean droplets use SSD storage.

#### GCP

* Use general-purpose [`t2d-standard`, `n2-standard`, or `n2d-standard`](https://cloud.google.com/compute/pricing#predefined_machine_types) VMs, or use [custom VMs](https://cloud.google.com/compute/docs/instances/creating-instance-with-custom-machine-type). For example, Cockroach Labs has used `t2d-standard-8`, `n2-standard-8`, and `n2d-standard-8` for performance benchmarking.

<Danger>
  Do not use `f1` or `g1` [shared-core machines](https://cloud.google.com/compute/docs/machine-types#sharedcore), which limit the load on CPU resources.
</Danger>

* Use [`pd-ssd` SSD persistent disks](https://cloud.google.com/compute/docs/disks/#pdspecs) or [local SSDs](https://cloud.google.com/compute/docs/disks/#localssds). Note that [the IOPS of SSD persistent disks depends both on the disk size and number of vCPUs on the machine](https://cloud.google.com/compute/docs/disks/performance#optimizessdperformance).
* `nobarrier` can be used with SSDs, but only if it has battery-backed write cache. Without one, data can be corrupted in the event of a crash.

  Cockroach Labs conducts most of our [internal performance tests](https://www.cockroachlabs.com/blog/2018_cloud_report/) using `nobarrier` to demonstrate the best possible performance, but understand that not all use cases can support this option.

## Security

An insecure cluster comes with serious risks:

* Your cluster is open to any client that can access any node's IP addresses.
* Any user, even `root`, can log in without providing a password.
* Any user, connecting as `root`, can read or write any data in your cluster.
* There is no network encryption or authentication, and thus no confidentiality.

Therefore, to deploy CockroachDB in production, it is strongly recommended to <InternalLink path="authentication">use TLS certificates to authenticate</InternalLink> the identity of nodes and clients and to encrypt data in transit between nodes and clients. You can use either the built-in <InternalLink path="cockroach-cert">`cockroach cert` commands</InternalLink> or <InternalLink path="create-security-certificates-openssl">`openssl` commands</InternalLink> to generate security certificates for your deployment. Regardless of which option you choose, you'll need the following files:

* A certificate authority (CA) certificate and key, used to sign all of the other certificates.
* A separate certificate and key for each node in your deployment, with the common name `node`.
* A separate certificate and key for each client and user you want to connect to your nodes, with the common name set to the username. The default user is `root`.

If you manage your own Certificate Authority (CA) infrastructure, CockroachDB supports mapping between the Subject field of your [X.509 certificates](https://en.wikipedia.org/wiki/X.509) and SQL <InternalLink path="security-reference/authorization#roles">roles</InternalLink>. For more information, see <InternalLink path="certificate-based-authentication-using-the-x509-subject-field">Certificate-based authentication using multiple values from the X.509 Subject field</InternalLink>.

Alternatively, CockroachDB supports <InternalLink path="authentication#client-authentication">password authentication</InternalLink>, although we typically recommend using client certificates instead.

For more information, see the <InternalLink path="security-reference/security-overview">Security Overview</InternalLink>.

## Networking

### Networking flags

When <InternalLink path="cockroach-start#flags-max-offset">starting a node</InternalLink>, two main flags are used to control its network connections:

* `--listen-addr` determines which address(es) to listen on for connections from other nodes and clients.
* `--advertise-addr` determines which address to tell other nodes to use.

The effect depends on how these two flags are used in combination:

|                                      | `--listen-addr` not specified                                                                                                                                    | `--listen-addr` specified                                                                                                                                                                                                                                                    |
| ------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **`--advertise-addr` not specified** | Node listens on all of its IP addresses on port `26257` and advertises its canonical hostname to other nodes.                                                    | Node listens on the IP address/hostname and port specified in `--listen-addr` and advertises this value to other nodes.                                                                                                                                                      |
| **`--advertise-addr` specified**     | Node listens on all of its IP addresses on port `26257` and advertises the value specified in `--advertise-addr` to other nodes. **Recommended for most cases.** | Node listens on the IP address/hostname and port specified in `--listen-addr` and advertises the value specified in `--advertise-addr` to other nodes. If the `--advertise-addr` port number is different than the one used in `--listen-addr`, port forwarding is required. |

<Tip>
  When using hostnames, make sure they resolve properly (e.g., via DNS or `etc/hosts`). In particular, be careful about the value advertised to other nodes, either via `--advertise-addr` or via `--listen-addr` when `--advertise-addr` is not specified.
</Tip>

### Cluster on a single network

When running a cluster on a single network, the setup depends on whether the network is private. In a private network, machines have addresses restricted to the network, not accessible to the public internet. Using these addresses is more secure and usually provides lower latency than public addresses.

| Private? | Recommended setup                                                                                                                                                                                                                                                                                                                                                                                                                      |
| -------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Yes      | Start each node with `--listen-addr` set to its private IP address and do not specify `--advertise-addr`. This will tell other nodes to use the private IP address advertised. Load balancers/clients in the private network must use it as well.                                                                                                                                                                                      |
| No       | Start each node with `--advertise-addr` set to a stable public IP address that routes to the node and do not specify `--listen-addr`. This will tell other nodes to use the specific IP address advertised, but load balancers/clients will be able to use any address that routes to the node.<br /><br />If load balancers/clients are outside the network, also configure firewalls to allow external traffic to reach the cluster. |

### Cluster spanning multiple networks

When running a cluster across multiple networks, the setup depends on whether nodes can reach each other across the networks.

| Nodes reachable across networks? | Recommended setup                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| -------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Yes                              | This is typical when all networks are on the same cloud. In this case, use the relevant [single network setup](#cluster-on-a-single-network) above.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| No                               | This is typical when networks are on different clouds. In this case, set up a [VPN](https://wikipedia.org/wiki/Virtual_private_network), [VPC](https://wikipedia.org/wiki/Virtual_private_cloud), [NAT](https://wikipedia.org/wiki/Network_address_translation), or another such solution to provide unified routing across the networks. Then start each node with `--advertise-addr` set to the address that is reachable from other networks and do not specify `--listen-addr`. This will tell other nodes to use the specific IP address advertised, but load balancers/clients will be able to use any address that routes to the node.<br /><br />Also, if a node is reachable from other nodes in its network on a private or local address, set <InternalLink path="cockroach-start#networking">`--locality-advertise-addr`</InternalLink> to that address. This will tell nodes within the same network to prefer the private or local address to improve performance. Note that this feature requires that each node is started with the <InternalLink path="cockroach-start#locality">`--locality`</InternalLink> flag. For more details, see this <InternalLink path="cockroach-start#start-a-multi-node-cluster-across-private-networks">example</InternalLink>. |

## Load balancing

Each CockroachDB node is an equally suitable SQL gateway to a cluster, but to ensure client performance and reliability, it's important to use load balancing:

* **Performance:** Load balancers spread client traffic across nodes. This prevents any one node from being overwhelmed by requests and improves overall cluster performance (queries per second).

* **Reliability:** Load balancers decouple client health from the health of a single CockroachDB node. To ensure that traffic is not directed to failed nodes or nodes that are not ready to receive requests, load balancers should use <InternalLink path="monitoring-and-alerting">CockroachDB's readiness health check</InternalLink>.

<Tip>
  With a single load balancer, client connections are resilient to node failure, but the load balancer itself is a point of failure. It's therefore best to make load balancing resilient as well by using multiple load balancing instances, with a mechanism like floating IPs or DNS to select load balancers for clients.
</Tip>

For guidance on load balancing, see the tutorial for your deployment environment:

| Environment                                                                                                        | Featured Approach                                   |
| ------------------------------------------------------------------------------------------------------------------ | --------------------------------------------------- |
| <InternalLink path="deploy-cockroachdb-on-premises#step-6-set-up-load-balancing">On-Premises</InternalLink>        | Use HAProxy.                                        |
| <InternalLink path="deploy-cockroachdb-on-aws#step-4-set-up-load-balancing">AWS</InternalLink>                     | Use Amazon's managed load balancing service.        |
| <InternalLink path="deploy-cockroachdb-on-microsoft-azure#step-4-set-up-load-balancing">Azure</InternalLink>       | Use Azure's managed load balancing service.         |
| <InternalLink path="deploy-cockroachdb-on-digital-ocean#step-3-set-up-load-balancing">Digital Ocean</InternalLink> | Use Digital Ocean's managed load balancing service. |
| <InternalLink path="deploy-cockroachdb-on-google-cloud-platform#step-4-set-up-load-balancing">GCP</InternalLink>   | Use GCP's managed TCP proxy load balancing service. |

## Connection pooling

Creating the appropriate size pool of connections is critical to gaining maximum performance in an application. Too few connections in the pool will result in high latency as each operation waits for a connection to open up. But adding too many connections to the pool can also result in high latency as each connection thread is being run in parallel by the system. The time it takes for many threads to complete in parallel is typically higher than the time it takes a smaller number of threads to run sequentially.

The number of **active** connections across all connection pools **should not exceed 4 times the number of vCPUs** in the cluster by a large amount. A connection is "active" when it is actively executing a query. To monitor active connections, use the <InternalLink path="connection-pooling#monitor-active-connections">**Active SQL Statements** graph and `sql.statements.active` metric</InternalLink>.

For guidance on sizing, validating, and using connection pools with CockroachDB, refer to <InternalLink path="connection-pooling">Use Connection Pools</InternalLink>.

To control the maximum number of non-superuser (<InternalLink path="security-reference/authorization">`root`</InternalLink> user or other <InternalLink path="security-reference/authorization#admin-role">`admin` role</InternalLink>) connections a <InternalLink path="architecture/sql-layer">gateway node</InternalLink> can have open at one time, use the `server.max_connections_per_gateway` <InternalLink path="cluster-settings">cluster setting</InternalLink>. If a new non-superuser connection would exceed this limit, the error message `"sorry, too many clients already"` is returned, along with error code `53300`.

## Monitoring and alerting

Despite CockroachDB's various <InternalLink path="frequently-asked-questions#how-does-cockroachdb-survive-failures">built-in safeguards against failure</InternalLink>, it is critical to actively monitor the overall health and performance of a cluster running in production and to create alerting rules that promptly send notifications when there are events that require investigation or intervention.

For details about available monitoring options and the most important events and metrics to alert on, see <InternalLink path="monitoring-and-alerting">Monitoring and Alerting</InternalLink>.

## Backup and restore

CockroachDB is purpose-built to be fault-tolerant and to recover automatically, but sometimes disasters happen. Having a <InternalLink path="disaster-recovery-planning">disaster recovery</InternalLink> plan enables you to recover quickly, while limiting the consequences.

Taking regular backups of your data in production is an operational best practice. You can create <InternalLink path="take-full-and-incremental-backups#full-backups">full</InternalLink> or <InternalLink path="take-full-and-incremental-backups#incremental-backups">incremental</InternalLink> backups of a cluster, database, or table. We recommend taking backups to <InternalLink path="use-cloud-storage">cloud storage</InternalLink> and enabling <InternalLink path="use-cloud-storage#immutable-storage">object locking</InternalLink> to protect the validity of your backups. CockroachDB supports Amazon S3, Azure Storage, and Google Cloud Storage for backups.

For details about available backup and restore types in CockroachDB, see <InternalLink path="backup-and-restore-overview#backup-and-restore-support">Backup and restore types</InternalLink>.

## Clock synchronization

CockroachDB requires moderate levels of clock synchronization to preserve data consistency. For this reason, when a node detects that its clock is out of sync with at least half of the other nodes in the cluster by 80% of the maximum offset allowed, it spontaneously shuts down. This offset defaults to 500ms but can be changed via the <InternalLink path="cockroach-start#flags-max-offset">`--max-offset`</InternalLink> flag when starting each node.

Regardless of clock skew, <InternalLink path="demo-serializable">`SERIALIZABLE`</InternalLink> and <InternalLink path="read-committed">`READ COMMITTED`</InternalLink> transactions both serve globally consistent ("non-stale") reads and <InternalLink path="developer-basics#how-transactions-work-in-cockroachdb">commit atomically</InternalLink>. However, skew outside the configured clock offset bounds can result in violations of single-key linearizability between causally dependent transactions. It's therefore important to prevent clocks from drifting too far by running [NTP](http://www.ntp.org/) or other clock synchronization software on each node.

In very rare cases, CockroachDB can momentarily run with a stale clock. This can happen when using vMotion, which can suspend a VM running CockroachDB, migrate it to different hardware, and resume it. This will cause CockroachDB to be out of sync for a short period before it jumps to the correct time. During this window, it would be possible for a client to read stale data and write data derived from stale reads. By enabling the `server.clock.forward_jump_check_enabled` <InternalLink path="cluster-settings">cluster setting</InternalLink>, you can be alerted when the CockroachDB clock jumps forward, indicating it had been running with a stale clock. To protect against this on vMotion, however, use the <InternalLink path="cockroach-start#general">`--clock-device`</InternalLink> flag to specify a [PTP hardware clock](https://www.kernel.org/doc/html/latest/driver-api/ptp.html) for CockroachDB to use when querying the current time. When doing so, you should not enable `server.clock.forward_jump_check_enabled` because forward jumps will be expected and harmless. For more information on how `--clock-device` interacts with vMotion, refer to [this blog post](https://web.archive.org/web/20210420062611/https://core.vmware.com/blog/cockroachdb-vmotion-support-vsphere-7-using-precise-timekeeping).

### Considerations

When setting up clock synchronization:

* All nodes in the cluster must be synced to the same time source, or to different sources that implement leap second smearing in the same way. For example, Google and Amazon have time sources that are compatible with each other (they implement [leap second smearing](https://developers.google.com/time/smear) in the same way), but are incompatible with the default NTP pool (which does not implement leap second smearing).
* For nodes running in AWS, we recommend [Amazon Time Sync Service](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/set-time.html#configure-amazon-time-service). For nodes running in GCP, we recommend [Google's internal NTP service](https://cloud.google.com/compute/docs/instances/configure-ntp#configure_ntp_for_your_instances). For nodes running elsewhere, we recommend [Google Public NTP](https://developers.google.com/time/). Note that the Google and Amazon time services can be mixed with each other, but they cannot be mixed with other time services (unless you have verified leap second behavior). Either all of your nodes should use the Google and Amazon services, or none of them should.
* If you do not want to use the Google or Amazon time sources, you can use [`chrony`](https://chrony.tuxfamily.org/index.html) and enable client-side leap smearing, unless the time source you're using already does server-side smearing. In most cases, we recommend the Google Public NTP time source because it handles smearing the leap second. If you use a different NTP time source that doesn't smear the leap second, you must configure client-side smearing manually and do so in the same way on each machine.
* Do not run more than one clock sync service on VMs where `cockroach` is running.
* For new clusters using the <InternalLink path="multiregion-overview">multi-region SQL abstractions</InternalLink>, Cockroach Labs recommends lowering the <InternalLink path="cockroach-start#flags-max-offset">`--max-offset`</InternalLink> setting to `250ms`. This setting is especially helpful for lowering the write latency of <InternalLink path="table-localities#global-tables">global tables</InternalLink>. Nodes can run with different values for `--max-offset`, but only for the purpose of updating the setting across the cluster using a rolling upgrade.

### Tutorials

For guidance on synchronizing clocks, see the tutorial for your deployment environment:

| Environment                                                                                                     | Featured Approach                                                                    |
| --------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| <InternalLink path="deploy-cockroachdb-on-premises#step-1-synchronize-clocks">On-Premises</InternalLink>        | Use NTP with Google's external NTP service.                                          |
| <InternalLink path="deploy-cockroachdb-on-aws#step-3-synchronize-clocks">AWS</InternalLink>                     | Use the Amazon Time Sync Service.                                                    |
| <InternalLink path="deploy-cockroachdb-on-microsoft-azure#step-3-synchronize-clocks">Azure</InternalLink>       | Disable Hyper-V time synchronization and use NTP with Google's external NTP service. |
| <InternalLink path="deploy-cockroachdb-on-digital-ocean#step-2-synchronize-clocks">Digital Ocean</InternalLink> | Use NTP with Google's external NTP service.                                          |
| <InternalLink path="deploy-cockroachdb-on-google-cloud-platform#step-3-synchronize-clocks">GCE</InternalLink>   | Use NTP with Google's internal NTP service.                                          |

## Cache and SQL memory size

CockroachDB manages its own memory caches, independently of the operating system. These are configured via the <InternalLink path="cockroach-start#flags">`--cache`</InternalLink> and <InternalLink path="cockroach-start#flags">`--max-sql-memory`</InternalLink> flags.

The default cache size is per-node and is passively consumed; it was chosen to facilitate development and testing, where users are likely to run multiple CockroachDB nodes on a single machine. Increasing the cache size will generally improve the node's read performance. Production systems should always configure this setting.

The <InternalLink path="cockroach-start#flags">`--cache`</InternalLink> flag controls the <InternalLink path="architecture/storage-layer#pebble">Pebble storage engine</InternalLink> block cache, which holds uncompressed blocks of persisted <InternalLink path="architecture/distribution-layer#overview">key-value data</InternalLink> in memory. If a read misses within the block cache, the storage engine reads the file via the operating system's page cache, which may hold the relevant block in-memory in its compressed form. Otherwise, the read is served from the storage device. The block cache fills to the configured size and is then recycled using a least-recently-used (LRU) policy.

Each node has a default SQL memory size of `25%`. This memory is used as-needed by active operations to store temporary data for SQL queries.

* Increasing a node's **cache size** will improve the node's read performance.
* Increasing a node's **SQL memory size** will increase the number of simultaneous client connections it allows, as well as the node's capacity for in-memory processing of rows when using `ORDER BY`, `GROUP BY`, `DISTINCT`, joins, and window functions.

<Note>
  SQL memory size applies a limit globally to all sessions at any point in time. Certain disk-spilling operations also respect a memory limit that applies locally to a single operation within a single query. This limit is configured via a separate cluster setting. For details, see <InternalLink path="vectorized-execution#disk-spilling-operations">Disk-spilling operations</InternalLink>.
</Note>

If a node runs out of its allocated SQL memory, a `memory budget exceeded` error occurs and the `cockroach` process may be at risk of crashing due to an out-of-memory (OOM) error. To mitigate this issue, refer to <InternalLink path="common-errors#memory-budget-exceeded">\`memory budget exceeded</InternalLink>.

To manually increase a node's cache size and SQL memory size, start the node using the <InternalLink path="cockroach-start#flags">`--cache`</InternalLink> and <InternalLink path="cockroach-start#flags">`--max-sql-memory`</InternalLink> flags. As long as all machines are [provisioned with sufficient RAM](#memory), you can experiment with increasing each value up to `35%`.

```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
cockroach start --cache=.35 --max-sql-memory=.35 {other start flags}
```

The default value for `--cache` is 128 MiB. For production deployments, set `--cache` to `25%` or higher. To determine appropriate settings for `--cache` and `--max-sql-memory`, use the following formula: <pre>`(2 * --max-sql-memory) + --cache <= 80% of system RAM
`</pre>

To help guard against <InternalLink path="cluster-setup-troubleshooting#out-of-memory-oom-crash">OOM events</InternalLink>, CockroachDB sets a soft memory limit using mechanisms in Go. Depending on your hardware and workload, you may not need to manually tune `--max-sql-memory`.

Test the configuration with a reasonable workload before deploying it to production.

<Note>
  On startup, if CockroachDB detects that `--max-sql-memory` or `--cache` are set too aggressively, a warning is logged.
</Note>

Because CockroachDB manages its own memory caches, Cockroach Labs recommends that you disable Linux memory swapping or allocate sufficient RAM to each node to prevent the node from running low on memory. Writing to swap is significantly less performant than writing to memory.

## Dependencies

The <InternalLink path="install-cockroachdb-linux">CockroachDB binary for Linux</InternalLink> depends on the following libraries:

| Library                                                | Description                                                                                                                                                                                                                                                                                                                                                                           |
| ------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [`glibc`](https://www.gnu.org/software/libc/libc.html) | The standard C library.<br /><br />If you build CockroachDB from source, rather than use the official CockroachDB binary for Linux, you can use any C standard library, including MUSL, the C standard library used on Alpine.                                                                                                                                                        |
| `libncurses`                                           | Required by the <InternalLink path="cockroach-sql">built-in SQL shell</InternalLink>.                                                                                                                                                                                                                                                                                                 |
| [`tzdata`](https://www.iana.org/time-zones)            | Required by certain features of CockroachDB that use time zone data, for example, to support using location-based names as time zone identifiers. This library is sometimes called `tz` or `zoneinfo`.<br /><br /> The CockroachDB binary now includes Go's copy of the `tzdata` library. If CockroachDB cannot find the `tzdata` library externally, it will use this built-in copy. |

These libraries are found by default on nearly all Linux distributions, with Alpine as the notable exception, but it's nevertheless important to confirm that they are installed and kept up-to-date. For the time zone data in particular, it's important for all nodes to have the same version; when updating the library, do so as quickly as possible across all nodes.

<Note>
  In Docker-based deployments of CockroachDB, these dependencies do not need to be manually addressed. The Docker image for CockroachDB includes them and keeps them up to date with each release of CockroachDB.
</Note>

## File descriptors limit

CockroachDB can use a large number of open file descriptors, often more than is available by default. Therefore, note the following recommendations.

For each CockroachDB node:

* At a **minimum**, the file descriptors limit must be `1956` (1700 per store plus 256 for networking). If the limit is below this threshold, the node will not start.
* It is **recommended** to set the file descriptors limit to `unlimited`; otherwise, the recommended limit is at least `15000` (10000 per store plus 5000 for networking). This higher limit ensures performance and accommodates cluster growth.
* When the file descriptors limit is not high enough to allocate the recommended amounts, CockroachDB allocates 10000 per store and the rest for networking; if this would result in networking getting less than 256, CockroachDB instead allocates 256 for networking and evenly splits the rest across stores.

### Increase the file descriptors limit

* [Yosemite and later](#yosemite-and-later)
* [Older versions](#older-versions)

#### Yosemite and later

To adjust the file descriptors limit for a single process in Mac OS X Yosemite and later, you must create a property list configuration file with the hard limit set to the recommendation mentioned [above](#file-descriptors-limit). Note that CockroachDB always uses the hard limit, so it's not technically necessary to adjust the soft limit, although we do so in the steps below.

For example, for a node with 3 stores, we would set the hard limit to at least 35000 (10000 per store and 5000 for networking) as follows:

1. Check the current limits:

   ```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
   $ launchctl limit maxfiles
   ```

   ```
   maxfiles    10240          10240
   ```

   The last two columns are the soft and hard limits, respectively. If `unlimited` is listed as the hard limit, note that the hidden default limit for a single process is actually 10240.

2. Create `/Library/LaunchDaemons/limit.maxfiles.plist` and add the following contents, with the final strings in the `ProgramArguments` array set to 35000:

   ```xml theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
   <?xml version="1.0" encoding="UTF-8"?>
   <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
     <plist version="1.0">
       <dict>
         <key>Label</key
           <string>limit.maxfiles</string
         <key>ProgramArguments</key
           <array>
             <string>launchctl</string
             <string>limit</string
             <string>maxfiles</string
             <string>35000</string
             <string>35000</string
           </array
         <key>RunAtLoad</key
           <true/>
         <key>ServiceIPC</key
           <false/>
       </dict
     </plist
   ```

   Make sure the plist file is owned by `root:wheel` and has permissions `-rw-r--r--`. These permissions should be in place by default.

3. Restart the system for the new limits to take effect.

4. Check the current limits:

   ```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
   $ launchctl limit maxfiles
   ```

   ```
   maxfiles    35000          35000
   ```

#### Older versions

To adjust the file descriptors limit for a single process in OS X versions earlier than Yosemite, edit `/etc/launchd.conf` and increase the hard limit to the recommendation mentioned [above](#file-descriptors-limit). Note that CockroachDB always uses the hard limit, so it's not technically necessary to adjust the soft limit, although we do so in the steps below.

For example, for a node with 3 stores, we would set the hard limit to at least 35000 (10000 per store and 5000 for networking) as follows:

1. Check the current limits:

   ```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
   $ launchctl limit maxfiles
   ```

   ```
   maxfiles    10240          10240
   ```

   The last two columns are the soft and hard limits, respectively. If `unlimited` is listed as the hard limit, note that the hidden default limit for a single process is actually 10240.

2. Edit (or create) `/etc/launchd.conf` and add a line that looks like the following, with the last value set to the new hard limit:

   ```
   limit maxfiles 35000 35000
   ```

3. Save the file, and restart the system for the new limits to take effect.

4. Verify the new limits:

   ```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
   $ launchctl limit maxfiles
   ```

   ```
   maxfiles    35000          35000
   ```

* [Per-Process Limit](#per-process-limit)
* [System-Wide Limit](#system-wide-limit)

#### Per-Process Limit

To adjust the file descriptors limit for a single process on Linux, enable PAM user limits and set the hard limit to the recommendation mentioned [above](#file-descriptors-limit). Note that CockroachDB always uses the hard limit, so it's not technically necessary to adjust the soft limit, although we do so in the steps below.

For example, for a node with 3 stores, we would set the hard limit to at least 35000 (10000 per store and 5000 for networking) as follows:

1. Make sure the following line is present in both `/etc/pam.d/common-session` and `/etc/pam.d/common-session-noninteractive`:

   ```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
   session    required   pam_limits.so
   ```

2. Set a limit for the number of open file descriptors. The specific limit you set depends on your workload and the hardware and configuration of your nodes.
   * **If you use `systemd`**, manually-set limits set using the `ulimit` command or a configuration file like `/etc/limits.conf` are ignored for services started by `systemd`. To limit the number of open file descriptors, add a line like the following to the service definition for the `cockroach` process. To allow an unlimited number of files, you can optionally set `LimitNOFILE` to `INFINITY`. Cockroach Labs recommends that you carefully test this configuration with a realistic workload before deploying it in production.

     ```none theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
     LimitNOFILE=35000
     ```

     Reload `systemd` for the new limit to take effect:

     ```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
     systemctl daemon-reload
     ```
   * **If you do not use `systemd`**: Edit `/etc/security/limits.conf` and append the following lines to the file:

     ```
     *              soft     nofile          35000
     *              hard     nofile          35000
     ```

     The `*` can be replaced with the username that will start CockroachDB.

     Save and close the file, then restart the system for the new limits to take effect.
     After the system restarts, verify the new limits:

     ```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
     ulimit -a
     ```

#### System-Wide Limit

You should also confirm that the file descriptors limit for the entire Linux system is at least 10 times higher than the per-process limit documented above (e.g., at least 150000).

1. **If you use `systemd`**, add a line like the following to the service definition for the `Manager` service. To allow an unlimited number of files, set `LimitNOFILE` to `INFINITY`.

   ```none theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
   LimitNOFILE=35000
   ```

   Reload `systemd` for the new limit to take effect:

   ```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
   systemctl daemon-reload
   ```

2. **If you do not use `systemd`**:
   1. Check the system-wide limit:

      ```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
      cat /proc/sys/fs/file-max
      ```
   2. If necessary, increase the system-wide limit in the `proc` file system:

      ```shell theme={"theme":{"light":"catppuccin-mocha","dark":"catppuccin-mocha"}}
      echo 150000 > /proc/sys/fs/file-max
      ```

CockroachDB for Windows is experimental and not supported in production. To learn about configuring limits on Windows, refer to the Microsoft community blog post [Pushing the Limits of Windows: Handles](https://techcommunity.microsoft.com/t5/windows-blog-archive/pushing-the-limits-of-windows-handles/ba-p/723848).

#### Attributions

This section, "File Descriptors Limit", is in part derivative of the chapter *Open File Limits* From the Riak LV 2.1.4 documentation, used under Creative Commons Attribution 3.0 Unported License.

## Orchestration / Kubernetes

When running CockroachDB on Kubernetes, making the following minimal customizations will result in better, more reliable performance:

* Use <InternalLink path="kubernetes-performance#disk-type">SSDs instead of traditional HDDs</InternalLink>.
* Configure CPU and memory <InternalLink path="kubernetes-performance#resource-requests-and-limits">resource requests and limits</InternalLink>.

For more information and additional customization suggestions, see our full detailed guide to <InternalLink path="kubernetes-performance">CockroachDB Performance on Kubernetes</InternalLink>.

## Transaction retries

When several transactions try to modify the same underlying data concurrently, they may experience <InternalLink path="performance-best-practices-overview#transaction-contention">contention</InternalLink> that leads to <InternalLink path="transactions#transaction-retries">transaction retries</InternalLink>. To avoid failures in production, your application should be engineered to handle transaction retries using <InternalLink path="transaction-retry-error-reference#client-side-retry-handling">client-side retry handling</InternalLink>.

