import React from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import Translate from '@docusaurus/Translate';
import {
  useActivePlugin,
  useDocVersionSuggestions,
  useDocsPreferredVersion,
  useDocsVersion,
} from '@docusaurus/plugin-content-docs/client';
import {ThemeClassNames} from '@docusaurus/theme-common';
import type {Props} from '@theme/DocVersionBanner';
import type {PropVersionMetadata} from '@docusaurus/plugin-content-docs';
import type {
  GlobalVersion,
  GlobalDoc,
} from '@docusaurus/plugin-content-docs/client';

// Swizzled (ejected) from @docusaurus/theme-classic to change two things:
//
// 1. The stock "unmaintained" copy says the version is "no longer actively
//    maintained", which overstates the state of v2.
// 2. The stock banner links only to the latest version. Readers landing on v2
//    also need the upgrade path, so we add a second link.
//
// Everything else matches upstream behaviour: link to the same doc in the
// latest version when it exists, and remember the version the reader picked.

const UPGRADE_GUIDE_PATH = '/docs/upgrade-guides/v3-breaking-changes';

// The site title is a long SEO string, so the upstream `{siteTitle}`
// interpolation reads badly in a banner. Use the short product name instead.
const PRODUCT_NAME = 'Rev-dep';

function UnreleasedVersionLabel({
  versionMetadata,
}: {
  versionMetadata: PropVersionMetadata;
}) {
  return (
    <Translate
      id="theme.docs.versions.unreleasedVersionLabel"
      description="The label used to tell the user that he's browsing an unreleased doc version"
      values={{
        siteTitle: PRODUCT_NAME,
        versionLabel: <b>{versionMetadata.label}</b>,
      }}>
      {
        'This is unreleased documentation for {siteTitle} {versionLabel} version.'
      }
    </Translate>
  );
}

function UnmaintainedVersionLabel({
  versionMetadata,
}: {
  versionMetadata: PropVersionMetadata;
}) {
  return (
    <Translate
      id="theme.docs.versions.unmaintainedVersionLabel"
      description="The label used to tell the user that he's browsing an older doc version"
      values={{
        siteTitle: PRODUCT_NAME,
        versionLabel: <b>{versionMetadata.label}</b>,
      }}>
      {'This is documentation for {siteTitle} {versionLabel}, a previous version.'}
    </Translate>
  );
}

const BannerLabelComponents = {
  unreleased: UnreleasedVersionLabel,
  unmaintained: UnmaintainedVersionLabel,
} as const;

function BannerLabel(props: {versionMetadata: PropVersionMetadata}) {
  const BannerLabelComponent =
    BannerLabelComponents[
      props.versionMetadata.banner as keyof typeof BannerLabelComponents
    ];
  return <BannerLabelComponent {...props} />;
}

function LatestVersionSuggestionLabel({
  versionLabel,
  to,
  onClick,
}: {
  versionLabel: string;
  to: string;
  onClick: () => void;
}) {
  return (
    <Translate
      id="theme.docs.versions.latestVersionSuggestionLabel"
      description="The label used to tell the user to check the latest version"
      values={{
        versionLabel,
        latestVersionLink: (
          <b>
            <Link to={to} onClick={onClick}>
              <Translate
                id="theme.docs.versions.latestVersionLinkLabel"
                description="The label used for the latest version suggestion link label">
                latest version
              </Translate>
            </Link>
          </b>
        ),
      }}>
      {'For up-to-date documentation, see the {latestVersionLink} ({versionLabel}).'}
    </Translate>
  );
}

function UpgradeGuideSuggestionLabel() {
  return (
    <Translate
      id="theme.docs.versions.upgradeGuideSuggestionLabel"
      description="The label used to point the user at the upgrade guide"
      values={{
        upgradeGuideLink: (
          <b>
            <Link to={UPGRADE_GUIDE_PATH}>
              <Translate
                id="theme.docs.versions.upgradeGuideLinkLabel"
                description="The label used for the upgrade guide suggestion link">
                upgrade guide
              </Translate>
            </Link>
          </b>
        ),
      }}>
      {'If you are upgrading, follow the {upgradeGuideLink}.'}
    </Translate>
  );
}

function DocVersionBannerEnabled({
  className,
  versionMetadata,
}: {
  className?: string;
  versionMetadata: PropVersionMetadata;
}) {
  // `failfast` guarantees a plugin is returned, but that is not encoded in the
  // return type, hence the assertion.
  const {pluginId} = useActivePlugin({failfast: true})!;
  const getVersionMainDoc = (version: GlobalVersion): GlobalDoc =>
    version.docs.find((doc) => doc.id === version.mainDocId)!;
  const {savePreferredVersionName} = useDocsPreferredVersion(pluginId);
  const {latestDocSuggestion, latestVersionSuggestion} =
    useDocVersionSuggestions(pluginId);
  // Try to link to same doc in latest version (not always possible), falling
  // back to main doc of latest version
  const latestVersionSuggestedDoc =
    latestDocSuggestion ?? getVersionMainDoc(latestVersionSuggestion);
  return (
    <div
      className={clsx(
        className,
        ThemeClassNames.docs.docVersionBanner,
        'alert alert--warning margin-bottom--md',
      )}
      role="alert">
      <div>
        <BannerLabel versionMetadata={versionMetadata} />
      </div>
      <div className="margin-top--md">
        <LatestVersionSuggestionLabel
          versionLabel={latestVersionSuggestion.label}
          to={latestVersionSuggestedDoc.path}
          onClick={() => savePreferredVersionName(latestVersionSuggestion.name)}
        />
      </div>
      <div className="margin-top--sm">
        <UpgradeGuideSuggestionLabel />
      </div>
    </div>
  );
}

export default function DocVersionBanner({className}: Props): React.ReactNode {
  const versionMetadata = useDocsVersion();
  if (versionMetadata.banner) {
    return (
      <DocVersionBannerEnabled
        className={className}
        versionMetadata={versionMetadata}
      />
    );
  }
  return null;
}
