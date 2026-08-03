import React from 'react';

export const Panel = ({ title, subtitle, description }) => (
  <section className="panel panel--bordered">
    <header className="panel__header">
      <h2 className="panel__title">{title}</h2>
      <p className="panel__subtitle">{subtitle}</p>
    </header>
    <div className="panel__body">{description}</div>
  </section>
);

export const InlineNested = () => <Row><Cell><Dot /></Cell></Row>;
