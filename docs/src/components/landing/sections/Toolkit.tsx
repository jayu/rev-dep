import { useRef, useState, type KeyboardEvent } from 'react';
import clsx from 'clsx';

import Section from '../primitives/Section';
import Split from '../primitives/Split';
import Terminal from '../primitives/Terminal';
import TrackedLink from '../primitives/TrackedLink';
import { trackClick } from '../analytics';
import { toolkitTabs } from '../data/toolkit';
import styles from './Toolkit.module.css';

/**
 * Which tab a key press moves to, or -1 for keys the tab list does not handle.
 *
 * Both axes are live because the list is vertical on desktop and horizontal on
 * a phone - the visitor should not have to know which one they are looking at.
 */
function nextIndex(key: string, current: number, count: number): number {
  const last = count - 1;
  switch (key) {
    case 'ArrowDown':
    case 'ArrowRight':
      return current === last ? 0 : current + 1;
    case 'ArrowUp':
    case 'ArrowLeft':
      return current === 0 ? last : current - 1;
    case 'Home':
      return 0;
    case 'End':
      return last;
    default:
      return -1;
  }
}

export default function Toolkit() {
  const [activeIndex, setActiveIndex] = useState(0);
  const tabRefs = useRef<(HTMLButtonElement | null)[]>([]);
  const active = toolkitTabs[activeIndex];

  /**
   * Both ways of choosing a tab report it - the arrow keys select as well as
   * move, so a keyboard visitor who arrows past three panels has looked at
   * three panels, the same as clicking them.
   */
  const selectTab = (index: number) => {
    setActiveIndex(index);
    trackClick({
      id: `toolkit_tab_${toolkitTabs[index].id}`,
      section: 'toolkit',
      type: 'toolkit_tab',
      target: toolkitTabs[index].command,
    });
  };

  // Arrow keys move focus and selection together, which is the expected
  // behaviour for a tab list whose panels are already rendered.
  const onKeyDown = (event: KeyboardEvent<HTMLButtonElement>, index: number) => {
    const next = nextIndex(event.key, index, toolkitTabs.length);
    if (next < 0) return;

    event.preventDefault();
    selectTab(next);
    tabRefs.current[next]?.focus();
  };

  return (
    <Section
      id="toolkit"
      tone="tinted"
      corner="top-right"
      eyebrow="Exploratory toolkit"
      title="Ask questions about your dependency graph"
      intro={
        <>
          Checks tell you something is wrong. These commands tell you why. Useful when a check
          fails, and before any refactor you are nervous about.
          <br />
          Teach your agent to reach for them and it can do the research itself - reading the real
          graph instead of guessing at it from the files it happens to have open.
        </>
      }
    >
      <Split left="20rem" right="1fr" gap="2rem">
        <div className={styles.tabs} role="tablist" aria-label="Exploratory commands">
          {toolkitTabs.map((tab, index) => (
            <button
              type="button"
              key={tab.id}
              role="tab"
              id={`toolkit-tab-${tab.id}`}
              aria-selected={index === activeIndex}
              aria-controls={`toolkit-panel-${tab.id}`}
              // Roving tabindex: one stop for the whole group, so Tab moves
              // past the list rather than through all seven of its buttons.
              tabIndex={index === activeIndex ? 0 : -1}
              ref={(node) => {
                tabRefs.current[index] = node;
              }}
              className={clsx(styles.tab, index === activeIndex && styles.tabActive)}
              onClick={() => selectTab(index)}
              onKeyDown={(event) => onKeyDown(event, index)}
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
          <TrackedLink
            className={styles.panelLink}
            to={active.docs}
            event={{
              id: `toolkit_docs_${active.id}`,
              section: 'toolkit',
              type: 'docs_link',
            }}
          >
            rev-dep {active.label} reference
          </TrackedLink>
        </div>
      </Split>
    </Section>
  );
}
