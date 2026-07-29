> ## Documentation Index
> Fetch the complete documentation index at: https://www.cockroachlabs.com/llms.txt
> Use this file to discover all available pages before exploring further.

# CockroachDB Releases Overview

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

This page describes how CockroachDB handles releases and links to the release notes for all CockroachDB [releases](#supported-releases).

## Release schedule

A new [major version](#major-versions) of CockroachDB is released quarterly. After a series of testing releases, each major version receives an initial production (GA) release, followed by [patch releases](#patch-releases). New versions first roll out to select CockroachDB Cloud organizations, and binaries are made available for CockroachDB self-hosted afterward:

* Initial production release (x.y.0 GA): Approximately 2 weeks after Cloud availability.
* Patch releases (x.y.1+): Approximately 1 week after Cloud availability.

Releases are named in the format `vYY.R.PP`, where `YY` indicates the year, `R` indicates the major version number starting with `1` each year, and `PP` indicates the patch version number.

For example, the `v25.4.6` production release is a patch release of major version <InternalLink version="releases" path="v25.4">`v25.4`</InternalLink>.

### Major versions

Major versions of CockroachDB alternate between **Regular** and **Innovation** releases:

* **Regular releases** offer extended support windows for long-term stability and security. This support window is further extended after a patch release is nominated to an <InternalLink version="releases" path="release-support-policy#support-types">**LTS** (long-term support)</InternalLink> release.
* **Innovation releases** offer shorter support windows but include all of the latest features.

For details on how LTS impacts support in CockroachDB self-hosted, refer to <InternalLink version="releases" path="release-support-policy">Release Support Policy</InternalLink>. For details on support per release type in CockroachDB Cloud, refer to <InternalLink version="cockroachcloud" path="upgrade-policy">CockroachDB Cloud Support and Upgrade Policy</InternalLink>.

### Patch releases

All major versions of CockroachDB receive patch releases that update functionality and fix issues. Patch releases after a major version's initial production release (`vYY.R.0`) are qualified for production environments.
The type and duration of support for a production release varies depending on the major release type, according to the <InternalLink version="releases" path="release-support-policy">release support policy</InternalLink>.

Prior to its initial production release, development releases are made available for testing and experimentation. These pre-production patch releases are sequenced in alpha (`vYY.R.0-alpha.1+`), followed by beta (`vYY.R.0-beta.1+`), followed by release candidate (`vYY.R.0-rc.1+`) versions. These releases are not supported for production environments.

<Danger>
  A cluster that is upgraded to an alpha binary of CockroachDB or a binary that was manually built from the `master` branch cannot subsequently be upgraded to a production release.
</Danger>

### CockroachDB Cloud releases

CockroachDB Cloud is a managed product that handles CockroachDB upgrades on your behalf, but still offers some flexibility over when to perform upgrades and which version to use. Version and upgrade support varies depending on the Cloud plan:

* CockroachDB Basic only supports **Regular releases**. Clusters are automatically upgraded to the next regular GA release, and receive regular patch updates.
* CockroachDB Standard only supports **Regular releases**. By default, clusters are automatically upgraded to the next regular GA release, with the option to handle the upgrade timing manually. Clusters receive regular patch updates.
* CockroachDB Advanced supports both **Regular** and **Innovation** releases.

For more information, read the <InternalLink version="cockroachcloud" path="upgrade-policy">CockroachDB Cloud upgrade policy</InternalLink>.

## Supported releases

| Version                                                            | Release Type | GA date    |
| ------------------------------------------------------------------ | ------------ | ---------- |
| <InternalLink version="releases" path="v26.2">v26.2</InternalLink> | Regular      | 2026-04-27 |
| <InternalLink version="releases" path="v26.1">v26.1</InternalLink> | Innovation   | 2026-02-02 |
| <InternalLink version="releases" path="v25.4">v25.4</InternalLink> | Regular      | 2025-11-03 |
| <InternalLink version="releases" path="v25.2">v25.2</InternalLink> | Regular      | 2025-05-12 |
| <InternalLink version="releases" path="v24.3">v24.3</InternalLink> | Regular      | 2024-11-18 |
| <InternalLink version="releases" path="v24.1">v24.1</InternalLink> | Regular      | 2024-05-20 |
| <InternalLink version="releases" path="v23.2">v23.2</InternalLink> | Regular      | 2024-02-05 |

## Upcoming releases

The following releases and their descriptions represent proposed plans that are subject to change. Please contact your account representative with any questions.

| Version | Release Type | Expected GA date |
| ------- | ------------ | ---------------- |
| v26.3   | Innovation   | 2026 Q3          |

## Archived releases

To access CockroachDB versions that are no longer supported, refer to <InternalLink version="releases" path="downloads-archive">CockroachDB Downloads Archive</InternalLink>.

## Licenses

All CockroachDB binaries released on or after the day 24.3.0 is released onward, including patch fixes for versions 23.1-24.2, are made available under the [CockroachDB Software License](https://www.cockroachlabs.com/cockroachdb-software-license).

All CockroachDB binaries released prior to the release date of 24.3.0 are variously licensed under the Business Source License 1.1 (BSL), the CockroachDB Community License (CCL), and other licenses specified in the source code.

To review the CCL, refer to the [CockroachDB Community License](https://www.cockroachlabs.com/cockroachdb-community-license) page.

In late 2024, Cockroach Labs retired its Core offering to consolidate on a single CockroachDB Enterprise offering under the CockroachDB Software License. This license is available at no charge for individual users and small businesses, and offers all users, free and paid, the full breadth of CockroachDB capabilities. For details, refer to the [CockroachDB licensing update](https://www.cockroachlabs.com/enterprise-license-update) and <InternalLink version="stable" path="licensing-faqs">Licensing FAQs</InternalLink>.

## Next steps

After choosing a version of CockroachDB, learn how to:

* <InternalLink version="cockroachcloud" path="create-your-cluster">Create a cluster in CockroachDB Cloud</InternalLink>.
* <InternalLink version="cockroachcloud" path="upgrade-cockroach-version">Upgrade a cluster in CockroachDB Cloud</InternalLink>.
* <InternalLink version="stable" path="install-cockroachdb">Install CockroachDB self-hosted</InternalLink>
* <InternalLink version="stable" path="upgrade-cockroach-version">Upgrade a Self-Hosted cluster</InternalLink>.

Be sure to review Cockroach Labs' <InternalLink version="releases" path="release-support-policy">Release Support Policy</InternalLink> and review information about applicable [software licenses](#licenses).
