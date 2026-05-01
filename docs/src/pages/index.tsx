import type { ReactNode } from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';

import styles from './index.module.css';
import testimonialCandidates from '../data/testimonialCandidates.json';

const TestimonialList = [
  testimonialCandidates.candidates[1],
  testimonialCandidates.candidates[2],
  testimonialCandidates.candidates[0],
];

function HomepageHeader() {
  const { siteConfig } = useDocusaurusContext();
  return (
    <header className={clsx('hero hero--primary', styles.heroBanner)}>
      <div className="container">
        <img
          src="img/logo-text.svg"
          alt="Rev-dep logo"
          className="mx-auto d-block"
          style={{ maxWidth: '50%', borderRadius: '8px', boxShadow: '0 4px 12px rgba(0,0,0,0.1)' }}
        />
        <Heading as="h1" className="hero__title" style={{color: 'white'}}>
          High-speed linter for your dependency graph.
        </Heading>
        <p className="hero__subtitle" style={{color: 'white'}}>
          Consolidate fragmented, sequential checks from
          multiple slow tools into a single, high-performance engine.
        </p>
        <div className={styles.buttons}>
          <Link
            className="button button--primary button--lg"
            to="/docs/intro">
            Get Started 🚀
          </Link>
        </div>
      </div>
    </header>
  );
}

function CodebaseScalingProblemSection() {
  return (
    <section className={clsx(styles.sectionPaddingLR, styles.marginLarge, styles.problemSection)}>
      <div className="container">
        <div className={styles.problemShell}>
          <div className={styles.problemIntro}>
            <p className={styles.sectionEyebrow}>About</p>
            <Heading as="h2">Codebase Scaling Problem</Heading>
            <p className={styles.problemLead}>
              As codebases scale, maintaining a mental map of dependencies becomes impossible.
            </p>
            <p className={styles.problemCopy}>
              <strong className="font-weight-bold">Rev-dep</strong> is a high-speed static analysis tool designed to enforce architecture integrity and dependency hygiene across large-scale JS/TS projects.
            </p>
            <p className={styles.problemCopy}>
              It consolidates fragmented, sequential checks from multiple slow tools into a single, high-performance engine that runs in one parallelized pass.
            </p>
          </div>
          <div className={styles.problemHighlightCard}>
            <p className={styles.problemHighlightLabel}>Dependency graph governance</p>
            <p className={styles.problemHighlightQuote}>
              Think of Rev-dep as a high-speed linter for your dependency graph.
            </p>
            <div className={styles.problemStatsRow}>
              <div className={styles.problemStat}>
                <span className={styles.problemStatValue}>500k+</span>
                <span className={styles.problemStatLabel}>lines of code audited</span>
              </div>
              <div className={styles.problemStat}>
                <span className={styles.problemStatValue}>~500ms</span>
                <span className={styles.problemStatLabel}>approximate runtime</span>
              </div>
            </div>
            <p className={styles.problemHighlightBody}>
              Implemented in <strong className="font-weight-bold">Go</strong> to bypass the performance bottlenecks of Node-based analysis, with CI-friendly speed that reduces developer wait states.
            </p>
          </div>
        </div>
        <div className={styles.problemGovernance}>
          <div className={styles.problemGovernanceHeader}>
            <Heading as="h3">Automated Codebase Governance</Heading>
            <p className={styles.problemGovernanceCopy}>
              Rev-dep moves beyond passive scanning to active enforcement, answering and failing CI for the hard questions that show up in growing codebases.
            </p>
          </div>
          <div className={styles.problemQuestionGrid}>
            <article className={styles.problemQuestionCard}>
              <Heading as="h4" className={styles.problemQuestionTitle}>Architecture Integrity</Heading>
              <p className={styles.problemQuestionBody}>"Is my 'Domain A' illegally importing from 'Domain B'?"</p>
            </article>
            <article className={styles.problemQuestionCard}>
              <Heading as="h4" className={styles.problemQuestionTitle}>Dead Code &amp; Bloat</Heading>
              <p className={styles.problemQuestionBody}>"Are these files unreachable, or are these `node_modules` unused?"</p>
            </article>
            <article className={styles.problemQuestionCard}>
              <Heading as="h4" className={styles.problemQuestionTitle}>Refactoring Safety</Heading>
              <p className={styles.problemQuestionBody}>"Which entry points actually use this utility, and are there circular chains?"</p>
            </article>
            <article className={styles.problemQuestionCard}>
              <Heading as="h4" className={styles.problemQuestionTitle}>Workspace Hygiene</Heading>
              <p className={styles.problemQuestionBody}>"Are my imports consistent and are all dependencies declared?"</p>
            </article>
          </div>
          <p className={styles.problemPanelFooter}>
            Rev-dep serves as a <strong className="font-weight-bold">high-speed gatekeeper</strong> for your CI, ensuring your dependency graph remains lean and your architecture stays intact as you iterate.
          </p>
        </div>
      </div>
    </section>
  );
}

type WhyRevDepFeature = {
  emoji: string;
  title: string;
  description: string;
};

const WhyRevDepFeatureList: WhyRevDepFeature[] = [
  {
    emoji: '🏗️',
    title: 'Monorepo Support',
    description: 'Designed for modern workspaces (pnpm, yarn, npm). Rev-dep natively resolves package.json exports/imports maps, TypeScript aliases and traces dependencies across package boundaries.',
  },
  {
    emoji: '🛡️',
    title: 'Config-Based Codebase Governance',
    description: 'Move beyond passive scanning. Use the configuration engine to enforce Module Boundaries and Import Conventions.',
  },
  {
    emoji: '🔍',
    title: 'Exploratory Toolkit',
    description: 'CLI toolkit that helps debug issues with dependencies between files. Understand transitive relation between files and fix issues.',
  },
  {
    emoji: '⚡',
    title: 'Unmatched Speed',
    description: 'Audit a 500k+ LoC project in approximately 500ms. 10x-200x faster execution than alternatives.',
  },
];

type Capability = {
  title: string;
  items: string[];
};

const CapabilityList: Capability[] = [
  {
    title: 'Governance and maintenance',
    items: [
      'Enforce module boundaries between domains and packages.',
      'Detect orphan files, circular imports, and unused exports.',
      'Catch missing or unused node modules before they hit CI.',
      'Apply import convention and autofix-friendly hygiene rules.',
    ],
  },
  {
    title: 'Exploratory analysis',
    items: [
      'Discover project entry points and inspect dependency trees.',
      'Trace who imports a file and resolve transitive dependency paths.',
      'Inspect circular chains and node module usage from the CLI.',
      'Use ad-hoc commands to debug architecture issues during refactors.',
    ],
  },
];

function WhyRevDepSection() {
  return (
    <section className={clsx(styles.sectionPadding, styles.marginLarge)}>
      <div className="container">
        <div className={clsx(styles.textCenter, styles.whyHeader)}>
          <Heading as="h2">Why Rev-dep? 🤔</Heading>
          <p className={styles.whySubtitle}>
            Built for teams that need fast dependency analysis and enforceable architectural rules without slowing down local development or CI.
          </p>
        </div>
        <div className={styles.featureGrid}>
          {WhyRevDepFeatureList.map((feature, idx) => (
            <article key={idx} className={styles.featureCard}>
              <div className={styles.featureEmoji}>{feature.emoji}</div>
              <Heading as="h3" className={styles.featureTitle}>{feature.title}</Heading>
              <p className={styles.featureDescription}>{feature.description}</p>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}

function TestimonialsSection() {
  return (
    <section className={clsx(styles.sectionPadding, styles.marginLarge, styles.testimonialsSection)}>
      <div className="container">
        <div className={styles.testimonialsHeader}>
          <p className={styles.sectionEyebrow}>Testimonials</p>
          <Heading as="h2">What early users are saying</Heading>
          <p className={styles.capabilitiesIntro}>
            Real comments pulled from public GitHub issue threads while teams evaluated rev-dep on production codebases.
          </p>
        </div>
        <div className={styles.testimonialsGrid}>
          {TestimonialList.map((testimonial) => (
            <article key={`${testimonial.author.login}-${testimonial.source.issueNumber}`} className={styles.testimonialCard}>
              <p className={styles.testimonialQuote}>“{testimonial.quote}”</p>
              <div className={styles.testimonialFooter}>
                <img
                  src={testimonial.author.avatarUrl}
                  alt={`${testimonial.author.name} avatar`}
                  className={styles.testimonialAvatar}
                />
                <div className={styles.testimonialMeta}>
                  <Link href={testimonial.author.profileUrl} className={styles.testimonialAuthor}>
                    {testimonial.author.name}
                  </Link>
                  <Link href={testimonial.source.commentUrl ?? testimonial.source.issueUrl} className={styles.testimonialSource}>
                    GitHub issue #{testimonial.source.issueNumber}
                  </Link>
                </div>
              </div>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}

function CapabilitiesSection() {
  return (
    <section className={clsx(styles.sectionPadding, styles.marginLarge, styles.capabilitiesSection)}>
      <div className="container">
        <div className={styles.capabilitiesHeader}>
          <p className={styles.sectionEyebrow}>Capabilities</p>
          <Heading as="h2">More than a single check runner</Heading>
          <p className={styles.capabilitiesIntro}>
            Rev-dep combines config-based governance with exploratory CLI tooling, so the same engine can enforce standards in CI and help engineers debug dependency problems locally.
          </p>
        </div>
        <div className={styles.capabilitiesGrid}>
          {CapabilityList.map((capability) => (
            <article key={capability.title} className={styles.capabilityCard}>
              <Heading as="h3" className={styles.capabilityTitle}>{capability.title}</Heading>
              <ul className={styles.capabilityList}>
                {capability.items.map((item) => (
                  <li key={item}>{item}</li>
                ))}
              </ul>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}

export default function Home(): ReactNode {
  return (
    <Layout
      // title="Rev-dep | High-Speed Dependency Graph Analysis for JS/TS Monorepos"
      // description="Enforce module boundaries, find circular imports, dead files, unused exports, and dependency issues in one fast CLI. Audit 500k+ LoC in around 500ms."
      >
      <HomepageHeader />
      <main>
        <CodebaseScalingProblemSection />
        <WhyRevDepSection />
        <TestimonialsSection />
        <CapabilitiesSection />
      </main>
    </Layout>
  );
}
