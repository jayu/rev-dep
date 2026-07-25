import React, {type ReactNode} from 'react';
import {
  PiBookOpen,
  PiDownloadSimple,
  PiGitFork,
  PiCube,
  PiListChecks,
  PiCompass,
  PiPuzzlePiece,
  PiLifebuoy,
  PiArrowCircleUp,
  PiArrowsLeftRight,
  PiScales,
  PiTerminalWindow,
  PiBookmarkSimple,
  PiChartLineUp,
  PiShieldCheck,
  PiGear,
  PiSliders,
} from 'react-icons/pi';
import type {IconType} from 'react-icons';

/**
 * Maps a `customProps.icon` string (set in sidebars.ts) to a Phosphor icon.
 * Keep the keys short and descriptive so the sidebar config stays readable.
 */
const ICONS: Record<string, IconType> = {
  overview: PiBookOpen,
  install: PiDownloadSimple,
  monorepo: PiGitFork,
  workspace: PiCube,
  checks: PiListChecks,
  explore: PiCompass,
  concepts: PiPuzzlePiece,
  troubleshooting: PiLifebuoy,
  upgrade: PiArrowCircleUp,
  migrate: PiArrowsLeftRight,
  compare: PiScales,
  cli: PiTerminalWindow,
  glossary: PiBookmarkSimple,
  telemetry: PiChartLineUp,
  shield: PiShieldCheck,
  gear: PiGear,
  sliders: PiSliders,
};

export function SidebarIcon({
  item,
  className,
}: {
  item: {customProps?: {[key: string]: unknown} | undefined};
  className?: string;
}): ReactNode {
  const key = item?.customProps?.icon;
  if (typeof key !== 'string') {
    return null;
  }
  const Icon = ICONS[key];
  if (!Icon) {
    return null;
  }
  return <Icon className={className} aria-hidden="true" />;
}
