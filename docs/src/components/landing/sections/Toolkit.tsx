import { useState } from 'react';
import Link from '@docusaurus/Link';
import clsx from 'clsx';

import Section from '../primitives/Section';
import Terminal from '../primitives/Terminal';
import { toolkitTabs } from '../data/toolkit';
import styles from './Toolkit.module.css';

export default function Toolkit() {
  const [activeId, setActiveId] = useState(toolkitTabs[0].id);
  const active = toolkitTabs.find((tab) => tab.id === activeId) ?? toolkitTabs[0];

  return (
    <Section
      id="toolkit"
      tone="tinted"
      eyebrow="Exploratory toolkit"
      title="Ask questions about your graph"
      intro="Checks tell you something is wrong. These commands tell you why. Useful when a check fails, and before any refactor you are nervous about."
    >
      <div className={styles.layout}>
        <div className={styles.tabs} role="tablist" aria-label="Exploratory commands">
          {toolkitTabs.map((tab) => (
            <button
              type="button"
              key={tab.id}
              role="tab"
              id={`toolkit-tab-${tab.id}`}
              aria-selected={tab.id === activeId}
              aria-controls={`toolkit-panel-${tab.id}`}
              className={clsx(styles.tab, tab.id === activeId && styles.tabActive)}
              onClick={() => setActiveId(tab.id)}
            >
              <span className={styles.tabCommand}>{tab.label}</span>
              <span className={styles.tabQuestion}>{tab.question}</span>
            </button>
          ))}
        </div>

        <div
          className={styles.panel}
          role="tabpanel"
          id={`toolkit-panel-${active.id}`}
          aria-labelledby={`toolkit-tab-${active.id}`}
        >
          <Terminal key={active.id} lines={active.lines} title={active.command} />
          <p className={styles.explanation}>{active.explanation}</p>
          <Link className={styles.panelLink} to={active.docs}>
            rev-dep {active.label} reference
          </Link>
        </div>
      </div>
    </Section>
  );
}
