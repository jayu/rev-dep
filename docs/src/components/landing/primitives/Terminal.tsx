import { useEffect, useRef, useState } from 'react';
import clsx from 'clsx';

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
  const ref = useRef<HTMLDivElement>(null);
  // Only a leading `$ …` line is typed; everything after it is output.
  const typedLine = lines[0]?.tone === 'prompt' ? lines[0].text ?? '' : null;
  const [typedChars, setTypedChars] = useState(0);
  const [playing, setPlaying] = useState(false);

  useEffect(() => {
    const node = ref.current;
    if (!node) return;

    const reduced = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
    if (reduced || typeof IntersectionObserver === 'undefined') {
      setTypedChars(typedLine ? typedLine.length : 0);
      setPlaying(true);
      return;
    }

    let timer: ReturnType<typeof setInterval> | undefined;

    const observer = new IntersectionObserver(
      (entries) => {
        if (!entries.some((e) => e.isIntersecting)) return;
        observer.disconnect();

        if (!typedLine) {
          setPlaying(true);
          return;
        }
        let i = 0;
        timer = setInterval(() => {
          i += 1;
          setTypedChars(i);
          if (i >= typedLine.length) {
            clearInterval(timer);
            setPlaying(true);
          }
        }, TYPE_MS);
      },
      { threshold: 0.05 },
    );

    observer.observe(node);
    return () => {
      observer.disconnect();
      if (timer) clearInterval(timer);
    };
  }, [typedLine]);

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
          const stillTyping = isTyped && typedChars < typedLine.length;

          return (
            <span
              key={i}
              className={clsx(
                styles.line,
                toneClass[line.tone ?? 'default'],
                indentClass[line.indent ?? 0],
                line.gap && styles.lineGap,
                // Output holds until the command has finished typing.
                !isTyped && (playing ? styles.lineIn : styles.linePending),
              )}
              style={
                !isTyped && playing ? { animationDelay: `${i * LINE_STAGGER_MS}ms` } : undefined
              }
            >
              {isTyped ? (
                <>
                  {typedLine.slice(0, typedChars)}
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
