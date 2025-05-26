DO $$
    BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'incidentimpactenum') THEN
            create type incidentimpactenum AS ENUM (
                'maintenance',
                'minor',
                'major',
                'outage'
                );
        END IF;
    END
$$;


CREATE TABLE IF NOT EXISTS component (
    id serial primary key,
    name character varying NOT NULL
);

CREATE INDEX IF NOT EXISTS ix_component_id ON component USING btree (id);

CREATE TABLE IF NOT EXISTS component_attribute (
    id serial primary key,
    component_id integer,
    name character varying NOT NULL,
    value character varying NOT NULL
);

CREATE INDEX IF NOT EXISTS ix_component_attribute_component_id ON component_attribute USING btree (component_id);
CREATE INDEX IF NOT EXISTS ix_component_attribute_id ON component_attribute USING btree (id);

CREATE TABLE IF NOT EXISTS incident (
    id serial primary key,
    text character varying NOT NULL,
    start_date timestamp without time zone NOT NULL,
    end_date timestamp without time zone,
    impact smallint NOT NULL,
    system boolean DEFAULT false NOT NULL
);

CREATE INDEX IF NOT EXISTS ix_incident_id ON incident USING btree (id);

CREATE TABLE IF NOT EXISTS incident_component_relation (
    incident_id integer NOT NULL,
    component_id integer NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS inc_comp_rel ON incident_component_relation USING btree (incident_id, component_id);

CREATE TABLE IF NOT EXISTS incident_status (
    id serial primary key,
    incident_id integer,
    "timestamp" timestamp without time zone NOT NULL,
    text character varying NOT NULL,
    status character varying NOT NULL
);

CREATE INDEX IF NOT EXISTS ix_incident_status_id ON incident_status USING btree (id);
CREATE INDEX IF NOT EXISTS ix_incident_status_incident_id ON incident_status USING btree (incident_id);

DO $$
    BEGIN
        IF NOT EXISTS (
            SELECT 1
            FROM pg_constraint
            WHERE conname = 'incident_component_relation_component_id_fkey'
        ) THEN
            ALTER TABLE ONLY incident_component_relation
                ADD CONSTRAINT incident_component_relation_component_id_fkey
                    FOREIGN KEY (component_id) REFERENCES component(id);
        END IF;
    END$$;



CREATE INDEX IF NOT EXISTS idx_incident_component_relation_component_id_incident_id ON incident_component_relation (component_id, incident_id);

CREATE INDEX IF NOT EXISTS idx_incident_status_incident_id_timestamp ON incident_status (incident_id, "timestamp");

DO $$
    BEGIN
        IF NOT EXISTS (
            SELECT 1
            FROM pg_constraint
            WHERE conname = 'unique_component_attribute'
        ) THEN
            ALTER TABLE component_attribute
                ADD CONSTRAINT unique_component_attribute
                    UNIQUE (component_id, name);
        END IF;
    END$$;

DO $$
    BEGIN
        IF NOT EXISTS (
            SELECT 1
            FROM pg_constraint
            WHERE conname = 'component_attribute_component_id_fkey'
        ) THEN
            ALTER TABLE ONLY component_attribute
                ADD CONSTRAINT component_attribute_component_id_fkey
                    FOREIGN KEY (component_id) REFERENCES component(id) ON DELETE CASCADE;
        END IF;
    END$$;

