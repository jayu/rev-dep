import type { ReactNode } from 'react';
import Layout from '@theme/Layout';

import Hero from '../components/landing/sections/Hero';
import SpeedChart from '../components/landing/sections/SpeedChart';
import Checks from '../components/landing/sections/Checks';
import AgentGuardrails from '../components/landing/sections/AgentGuardrails';
import ReplacesStack from '../components/landing/sections/ReplacesStack';
import Compatibility from '../components/landing/sections/Compatibility';
import Adoption from '../components/landing/sections/Adoption';
import Toolkit from '../components/landing/sections/Toolkit';
import Testimonials from '../components/landing/sections/Testimonials';
import RealProjects from '../components/landing/sections/RealProjects';
import Faq from '../components/landing/sections/Faq';
import FinalCta from '../components/landing/sections/FinalCta';

/**
 * The landing page is only composition - every section owns its own markup,
 * styles and copy. To reorder the page, move a line. To edit wording, open the
 * section's file (or its data module under components/landing/data).
 */
export default function Home(): ReactNode {
  return (
    <Layout>
      <Hero />
      <main>
        <SpeedChart />
        <Checks />
        <AgentGuardrails />
        <ReplacesStack />
        <Compatibility />
        <Adoption />
        <Toolkit />
        <Testimonials />
        <RealProjects />
        <Faq />
        <FinalCta />
      </main>
    </Layout>
  );
}
