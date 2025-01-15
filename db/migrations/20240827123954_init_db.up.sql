CREATE TYPE IF NOT EXISTS incidentimpactenum AS ENUM (
    'maintenance',
    'minor',
    'major',
    'outage'
    );

CREATE TABLE IF NOT EXISTS component (
    id serial primary key,
    name character varying NOT NULL
);

CREATE INDEX ix_component_id ON component USING btree (id);

CREATE TABLE IF NOT EXISTS component_attribute (
    id serial primary key,
    component_id integer,
    name character varying NOT NULL,
    value character varying NOT NULL
);

CREATE INDEX ix_component_attribute_component_id ON component_attribute USING btree (component_id);
CREATE INDEX ix_component_attribute_id ON component_attribute USING btree (id);

CREATE TABLE IF NOT EXISTS incident (
    id serial primary key,
    text character varying NOT NULL,
    start_date timestamp without time zone NOT NULL,
    end_date timestamp without time zone,
    impact smallint NOT NULL,
    system boolean DEFAULT false NOT NULL
);

CREATE INDEX ix_incident_id ON incident USING btree (id);

CREATE TABLE IF NOT EXISTS incident_component_relation (
    incident_id integer NOT NULL,
    component_id integer NOT NULL
);

CREATE UNIQUE INDEX inc_comp_rel ON incident_component_relation USING btree (incident_id, component_id);

CREATE TABLE IF NOT EXISTS incident_status (
    id serial primary key,
    incident_id integer,
    "timestamp" timestamp without time zone NOT NULL,
    text character varying NOT NULL,
    status character varying NOT NULL
);

CREATE INDEX ix_incident_status_id ON incident_status USING btree (id);
CREATE INDEX ix_incident_status_incident_id ON incident_status USING btree (incident_id);


ALTER TABLE ONLY component_attribute
    ADD CONSTRAINT component_attribute_component_id_fkey FOREIGN KEY (component_id) REFERENCES component(id);

ALTER TABLE ONLY incident_component_relation
    ADD CONSTRAINT incident_component_relation_component_id_fkey FOREIGN KEY (component_id) REFERENCES component(id);

ALTER TABLE ONLY incident_component_relation
    ADD CONSTRAINT incident_component_relation_incident_id_fkey FOREIGN KEY (incident_id) REFERENCES incident(id);

ALTER TABLE ONLY incident_status
    ADD CONSTRAINT incident_status_incident_id_fkey FOREIGN KEY (incident_id) REFERENCES incident(id);

CREATE INDEX idx_incident_component_relation_component_id_incident_id ON incident_component_relation (component_id, incident_id);

CREATE INDEX idx_incident_status_incident_id_timestamp ON incident_status (incident_id, "timestamp");

ALTER TABLE component_attribute ADD CONSTRAINT unique_component_attribute UNIQUE (component_id, name);

ALTER TABLE incident_status ADD CONSTRAINT unique_incident_status UNIQUE (incident_id, "timestamp");

ALTER TABLE component_attribute
    ADD CONSTRAINT component_attribute_component_id_fkey
    FOREIGN KEY (component_id) REFERENCES component(id) ON DELETE CASCADE;

ALTER TABLE incident_component_relation
    ADD CONSTRAINT incident_component_relation_incident_id_fkey
    FOREIGN KEY (incident_id) REFERENCES incident(id) ON DELETE CASCADE;

ALTER TABLE incident_status
    ADD CONSTRAINT incident_status_incident_id_fkey
    FOREIGN KEY (incident_id) REFERENCES incident(id) ON DELETE CASCADE;
