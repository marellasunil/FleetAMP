import React from 'react';
import Layout from '@theme/Layout';
import Link from '@docusaurus/Link';
import styles from './index.module.css';

const features = [
  ['Fleet visibility', 'Build a central inventory of managed telemetry agents, their identity, health, capabilities, and last-seen state.'],
  ['OpAMP-first', 'Use an isolated OpAMP management adapter while keeping FleetAMP domain, API, storage, and integrations independent of the wire protocol.'],
  ['Configuration control', 'Evolve toward desired/effective configuration, validated remote configuration, groups, selectors, and controlled rollouts.'],
  ['Extensible by design', 'Provider boundaries leave room for Git, CSDM/CMDB enrichment, identity, additional telemetry agents, and deployment platforms.'],
];

export default function Home() {
  return (
    <Layout title="Open telemetry fleet management" description="FleetAMP documentation">
      <main>
        <section className={styles.hero}>
          <div className="container">
            <p className={styles.eyebrow}>OPEN SOURCE · OPAMP · VENDOR NEUTRAL</p>
            <h1>Operate telemetry collectors as a fleet.</h1>
            <p className={styles.lead}>FleetAMP is an open-source control plane for discovering, observing, configuring, and operating telemetry agents without coupling fleet management to a telemetry backend.</p>
            <div className={styles.actions}>
              <Link className="button button--primary button--lg" to="/docs/getting-started/overview">Get started</Link>
              <Link className="button button--secondary button--lg" href="https://github.com/marellasunil/FleetAMP">View on GitHub</Link>
            </div>
          </div>
        </section>
        <section className={styles.section}>
          <div className="container">
            <div className={styles.grid}>{features.map(([title, text]) => <article className={styles.card} key={title}><h3>{title}</h3><p>{text}</p></article>)}</div>
          </div>
        </section>
        <section className={styles.sectionAlt}>
          <div className="container">
            <h2>One core. Multiple environments.</h2>
            <p>FleetAMP models managed agents independently from where they run. VM, bare metal, containers, and Kubernetes are deployment contexts rather than separate products.</p>
            <pre className={styles.diagram}>{`Git / CMDB / Identity\n        ↓\n     FleetAMP\n  API · Policy · State\n        ↓\n Management adapters\n        ↓\n OTel Collectors / future agents\n        ↓\n Your telemetry backends`}</pre>
          </div>
        </section>
      </main>
    </Layout>
  );
}
