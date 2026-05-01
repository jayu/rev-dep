import type {ReactNode} from 'react';
import clsx from 'clsx';
import Heading from '@theme/Heading';
import styles from './styles.module.css';

type FeatureItem = {
  title: string;
  svgPath: any;
  description: ReactNode;
};

const FeatureList: FeatureItem[] = [
  {
    title: 'Unmatched Speed',
    svgPath: require('@site/static/img/undraw_docusaurus_mountain.svg').default,
    description: (
      <>Audit a 500k+ LoC project in approximately 500ms. 10x-200x faster execution than alternatives.</>
    ),
  },
  {
    title: 'Automated Governance',
    svgPath: require('@site/static/img/undraw_docusaurus_tree.svg').default,
    description: (
      <>Enforce module boundaries, import conventions, and detect unused exports/files automatically.</>
    ),
  },
  {
    title: 'Monorepo Support',
    svgPath: require('@site/static/img/undraw_docusaurus_react.svg').default,
    description: (
      <>Designed for pnpm, yarn, and npm. Natively resolves package.json exports/imports and TypeScript aliases.</>
    ),
  },
];

function Feature({title, svgPath, description}: FeatureItem) {
  const Svg = svgPath;
  return (
    <div className={clsx('col col--4')}>
      <div className="text-center">
        <Svg className={styles.featureSvg} role="img" />
      </div>
      <div className="text-center padding-horiz--md">
        <Heading as="h3">{title}</Heading>
        <p>{description}</p>
      </div>
    </div>
  );
}

export default function HomepageFeatures(): ReactNode {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}
