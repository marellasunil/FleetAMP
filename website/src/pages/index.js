import React from 'react';
import Layout from '@theme/Layout';
import Link from '@docusaurus/Link';
import styles from './index.module.css';

const capabilities = [
  {
    icon: '◎',
    title: 'Fleet visibility',
    text: 'A protocol-independent inventory model for telemetry agents, health, identity, capabilities, labels, and deployment context.',
    status: 'Foundation',
  },
  {
    icon: '⇄',
    title: 'OpAMP control plane',
    text: 'An isolated OpAMP adapter keeps protocol details away from FleetAMP APIs, storage, policy, and future integrations.',
    status: 'In progress',
  },
  {
    icon: '⌘',
    title: 'Configuration control',
    text: 'Designed for validated remote configuration, desired/effective state, group targeting, and controlled rollouts.',
    status: 'Planned',
  },
  {
    icon: '◇',
    title: 'Extensible integrations',
    text: 'Provider boundaries for Git, CSDM/CMDB enrichment, identity, additional telemetry agents, and deployment platforms.',
    status: 'Designed',
  },
];

const roadmap = [
  ['01', 'Connect', 'OpenTelemetry Supervisors connect to the FleetAMP management endpoint.'],
  ['02', 'Discover', 'FleetAMP normalizes identity, metadata, health, capabilities, and runtime context.'],
  ['03', 'Control', 'Desired configuration is validated, versioned, targeted, and delivered through a management adapter.'],
  ['04', 'Observe', 'FleetAMP compares desired and effective state and surfaces fleet health and deployment results.'],
];

function Status({ children }) {
  return <span className={styles.status}>{children}</span>;
}

export default function Home() {
  return (
    <Layout title="Open telemetry fleet management" description="FleetAMP — open, extensible fleet management for telemetry agents">
      <main>
        <section className={styles.hero}>
          <div className={`container ${styles.heroGrid}`}>
            <div className={styles.heroCopy}>
              <div className={styles.badge}>OPEN SOURCE · OPAMP-FIRST · VENDOR NEUTRAL</div>
              <h1>Operate telemetry collectors <span>as a fleet.</span></h1>
              <p className={styles.lead}>
                FleetAMP is an open control plane for discovering, observing, configuring, and operating telemetry agents without coupling fleet management to a telemetry backend.
              </p>
              <div className={styles.actions}>
                <Link className="button button--primary button--lg" to="/docs/getting-started/overview">Explore the docs</Link>
                <Link className={styles.secondaryButton} href="https://github.com/marellasunil/FleetAMP">GitHub ↗</Link>
              </div>
              <div className={styles.heroMeta}>
                <span>Apache-2.0</span>
                <span>Self-hosted</span>
                <span>Go</span>
                <span>Community project</span>
              </div>
            </div>

            <div className={styles.consoleWrap}>
              <div className={styles.glow}></div>
              <div className={styles.console}>
                <div className={styles.consoleTop}>
                  <div className={styles.dots}><i></i><i></i><i></i></div>
                  <span>fleetamp / control-plane</span>
                  <span className={styles.live}>● architecture</span>
                </div>
                <div className={styles.consoleBody}>
                  <div className={styles.layer}><span>Sources</span><b>Git · CMDB · Identity</b></div>
                  <div className={styles.arrow}>↓</div>
                  <div className={`${styles.layer} ${styles.primaryLayer}`}><span>FleetAMP</span><b>API · Policy · State · Inventory</b></div>
                  <div className={styles.arrow}>↓</div>
                  <div className={styles.layer}><span>Management</span><b>OpAMP · future adapters</b></div>
                  <div className={styles.arrow}>↓</div>
                  <div className={styles.agentRow}>
                    <div><small>AGENT</small><strong>OTel Collector</strong><em>VM</em></div>
                    <div><small>GATEWAY</small><strong>OTel Collector</strong><em>Kubernetes</em></div>
                    <div><small>FUTURE</small><strong>Other agents</strong><em>Adapter</em></div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>

        <section className={styles.signalBar}>
          <div className={`container ${styles.signalGrid}`}>
            <div><strong>One domain model</strong><span>Across VM, container and Kubernetes runtimes</span></div>
            <div><strong>Adapter isolation</strong><span>Upstream protocol changes stay at the boundary</span></div>
            <div><strong>Backend neutral</strong><span>Fleet management stays independent from telemetry storage</span></div>
          </div>
        </section>

        <section className={styles.section}>
          <div className="container">
            <div className={styles.sectionHeading}>
              <div><p className={styles.kicker}>PLATFORM PRINCIPLES</p><h2>Built as a control plane, not a fork.</h2></div>
              <p>FleetAMP prefers composition over custom distributions. Upstream agents and protocols remain replaceable dependencies behind explicit adapters.</p>
            </div>
            <div className={styles.cardGrid}>
              {capabilities.map((item) => (
                <article className={styles.card} key={item.title}>
                  <div className={styles.cardTop}><span className={styles.icon}>{item.icon}</span><Status>{item.status}</Status></div>
                  <h3>{item.title}</h3>
                  <p>{item.text}</p>
                </article>
              ))}
            </div>
          </div>
        </section>

        <section className={styles.darkSection}>
          <div className="container">
            <div className={styles.sectionHeadingDark}>
              <div><p className={styles.kicker}>CONTROL LOOP</p><h2>From connection to desired state.</h2></div>
              <p>The first FleetAMP milestones focus on a small, verifiable management loop before broader integrations are added.</p>
            </div>
            <div className={styles.roadmapGrid}>
              {roadmap.map(([number, title, text]) => (
                <article className={styles.roadmapCard} key={number}>
                  <span>{number}</span><h3>{title}</h3><p>{text}</p>
                </article>
              ))}
            </div>
          </div>
        </section>

        <section className={styles.section}>
          <div className="container">
            <div className={styles.split}>
              <div>
                <p className={styles.kicker}>EXTENSIBLE BY DESIGN</p>
                <h2>Keep integrations optional.</h2>
                <p className={styles.bodyText}>FleetAMP keeps integration contracts in the architecture now without making external systems mandatory for the first release.</p>
                <Link to="/docs/integrations/overview" className={styles.textLink}>Explore integration design →</Link>
              </div>
              <div className={styles.integrationGrid}>
                <div><b>Configuration</b><span>Azure DevOps · GitHub · GitLab · Filesystem</span></div>
                <div><b>Enrichment</b><span>CSDM / CMDB · REST · custom metadata</span></div>
                <div><b>Identity</b><span>OIDC · scoped authorization · RBAC</span></div>
                <div><b>Management</b><span>OpAMP first · additional adapters later</span></div>
              </div>
            </div>
          </div>
        </section>

        <section className={styles.cta}>
          <div className="container">
            <div className={styles.ctaBox}>
              <div><p className={styles.kicker}>COMMUNITY EDITION</p><h2>Follow the build from the foundation up.</h2><p>FleetAMP is early-stage. The docs distinguish implemented foundations from planned capabilities as the project evolves.</p></div>
              <div className={styles.ctaActions}><Link className="button button--primary button--lg" to="/docs/roadmap">View roadmap</Link><Link className={styles.secondaryButton} href="https://github.com/marellasunil/FleetAMP">Contribute ↗</Link></div>
            </div>
          </div>
        </section>
      </main>
    </Layout>
  );
}
