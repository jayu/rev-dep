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
  parts?: { text: string; tone?: LineTone }[];
  tone?: LineTone;
  /** Indent level; 1 unit ≈ one nesting step in the CLI's own output. */
  indent?: 0 | 1 | 2;
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

/**
 * A terminal window rendered as text rather than an image, so it stays crisp,
 * selectable and searchable, and costs no image weight.
 */
export default function Terminal({ title, lines, className }: TerminalProps) {
  return (
    <div className={clsx(styles.terminal, className)}>
      <div className={styles.chrome}>
        <span className={styles.dotRed} />
        <span className={styles.dotAmber} />
        <span className={styles.dotGreen} />
        {title && <span className={styles.chromeTitle}>{title}</span>}
      </div>
      <pre className={styles.body}>
        {lines.map((line, i) => (
          <span
            key={i}
            className={clsx(styles.line, toneClass[line.tone ?? 'default'], indentClass[line.indent ?? 0])}
          >
            {line.parts
              ? line.parts.map((part, j) => (
                  <span key={j} className={toneClass[part.tone ?? 'default']}>
                    {part.text}
                  </span>
                ))
              : line.text || ' '}
          </span>
        ))}
      </pre>
    </div>
  );
}
