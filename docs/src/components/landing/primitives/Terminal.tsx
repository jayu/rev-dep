import { useEffect, useState } from 'react';
import clsx from 'clsx';

import useInView from './useInView';
import { prefersReducedMotion, useIsomorphicLayoutEffect } from './motion';
import styles from './Terminal.module.css';

/** Colour role for a single output line. */
export type LineTone = 'default' | 'dim' | 'ok' | 'bad' | 'accent' | 'prompt';

export type TerminalLine = {
  text?: string;
  /**
   * Use instead of `text` to colour parts of one line differently - e.g. to
   * highlight only the file count inside an otherwise plain line.
   */
  parts?: { text: string; tone?: LineTone; strong?: boolean }[];
  tone?: LineTone;
  /** Indent level; 1 unit ≈ one nesting step in the CLI's own output. */
  indent?: 0 | 1 | 2;
  /**
   * Renders a blank line at a fraction of its height. The CLI separates blocks
   * with a real newline, but at these font sizes a full empty line reads as a
   * bigger gap on screen than it does in a terminal - so separators can be
   * tightened without dropping them.
   */
  gap?: boolean;
};

type TerminalProps = {
  /** Shown in the window chrome. Usually the command being run. */
  title?: string;
  lines: TerminalLine[];
  className?: string;
};

const toneClass: Record<LineTone, string> = {
  default: styles.lineDefault,
  dim: styles.lineDim,
  ok: styles.lineOk,
  bad: styles.lineBad,
  accent: styles.lineAccent,
  prompt: styles.linePrompt,
};

const indentClass = [styles.indent0, styles.indent1, styles.indent2];

const TYPE_MS = 13;
const LINE_STAGGER_MS = 28;

/**
 * `shown` is the finished transcript with no animation - what the server
 * renders, and where the component stays if JavaScript never runs or motion is
 * turned down. A terminal that is invisible until a client-side observer fires
 * would leave the hero's main piece of evidence out of the static HTML.
 *
 * Only after mount does it rewind to `pending` and wait to be scrolled to.
 */
type Phase = 'shown' | 'pending' | 'playing';

/**
 * A terminal window rendered as text rather than an image, so it stays crisp,
 * selectable and searchable, and costs no image weight.
 *
 * It plays once when scrolled into view: the command line types out character
 * by character, then the output streams in a line at a time - the order a real
 * terminal produces it. Remounting (switching toolkit tabs via `key`) replays
 * it; hovering does not, because a replay under the cursor while you are
 * reading is more distracting than it is charming.
 */
export default function Terminal({ title, lines, className }: TerminalProps) {
  // Only a leading `$ …` line is typed; everything after it is output.
  const typedLine = lines[0]?.tone === 'prompt' ? lines[0].text ?? '' : null;
  const [phase, setPhase] = useState<Phase>('shown');
  const [typedChars, setTypedChars] = useState(0);
  const { ref, inView } = useInView<HTMLDivElement>({ enabled: phase === 'pending' });

  // Before the browser paints, so the finished transcript never flashes up and
  // then rewinds.
  useIsomorphicLayoutEffect(() => {
    if (prefersReducedMotion()) return;
    setPhase('pending');
    setTypedChars(0);
  }, []);

  useEffect(() => {
    if (phase !== 'pending' || !inView) return;

    if (!typedLine) {
      setPhase('playing');
      return;
    }

    let i = 0;
    const timer = setInterval(() => {
      i += 1;
      setTypedChars(i);
      if (i >= typedLine.length) {
        clearInterval(timer);
        setPhase('playing');
      }
    }, TYPE_MS);

    return () => clearInterval(timer);
  }, [phase, inView, typedLine]);

  return (
    <div className={clsx(styles.terminal, className)} ref={ref}>
      <div className={styles.chrome}>
        <span className={styles.dotRed} />
        <span className={styles.dotAmber} />
        <span className={styles.dotGreen} />
        {title && <span className={styles.chromeTitle}>{title}</span>}
      </div>
      <pre className={styles.body}>
        {lines.map((line, i) => {
          const isTyped = typedLine !== null && i === 0;
          const shownChars = phase === 'shown' ? typedLine?.length ?? 0 : typedChars;
          const stillTyping = isTyped && shownChars < typedLine.length;

          return (
            <span
              key={i}
              className={clsx(
                styles.line,
                toneClass[line.tone ?? 'default'],
                indentClass[line.indent ?? 0],
                line.gap && styles.lineGap,
                // Output holds until the command has finished typing.
                !isTyped && phase === 'pending' && styles.linePending,
                !isTyped && phase === 'playing' && styles.lineIn,
              )}
              style={
                !isTyped && phase === 'playing'
                  ? { animationDelay: `${i * LINE_STAGGER_MS}ms` }
                  : undefined
              }
            >
              {isTyped ? (
                <>
                  {typedLine.slice(0, shownChars)}
                  {stillTyping && <span className={styles.cursor} />}
                </>
              ) : line.parts ? (
                line.parts.map((part, j) => (
                  <span
                    key={j}
                    className={clsx(toneClass[part.tone ?? 'default'], part.strong && styles.strong)}
                  >
                    {part.text}
                  </span>
                ))
              ) : (
                line.text || ' '
              )}
            </span>
          );
        })}
      </pre>
    </div>
  );
}
