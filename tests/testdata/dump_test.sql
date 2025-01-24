--
-- PostgreSQL database dump
--

-- Dumped from database version 15.8 (Debian 15.8-1.pgdg120+1)
-- Dumped by pg_dump version 16.4

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: incidentimpactenum; Type: TYPE; Schema: public; Owner: pg
--

CREATE TYPE public.incidentimpactenum AS ENUM (
    'maintenance',
    'minor',
    'major',
    'outage'
);


ALTER TYPE public.incidentimpactenum OWNER TO pg;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: component; Type: TABLE; Schema: public; Owner: pg
--

CREATE TABLE public.component (
    id integer NOT NULL,
    name character varying NOT NULL
);


ALTER TABLE public.component OWNER TO pg;

--
-- Name: component_attribute; Type: TABLE; Schema: public; Owner: pg
--

CREATE TABLE public.component_attribute (
    id integer NOT NULL,
    component_id integer,
    name character varying NOT NULL,
    value character varying NOT NULL
);


ALTER TABLE public.component_attribute OWNER TO pg;

--
-- Name: component_attribute_id_seq; Type: SEQUENCE; Schema: public; Owner: pg
--

CREATE SEQUENCE public.component_attribute_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.component_attribute_id_seq OWNER TO pg;

--
-- Name: component_attribute_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: pg
--

ALTER SEQUENCE public.component_attribute_id_seq OWNED BY public.component_attribute.id;


--
-- Name: component_id_seq; Type: SEQUENCE; Schema: public; Owner: pg
--

CREATE SEQUENCE public.component_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.component_id_seq OWNER TO pg;

--
-- Name: component_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: pg
--

ALTER SEQUENCE public.component_id_seq OWNED BY public.component.id;


--
-- Name: incident; Type: TABLE; Schema: public; Owner: pg
--

CREATE TABLE public.incident (
    id integer NOT NULL,
    text character varying NOT NULL,
    start_date timestamp without time zone NOT NULL,
    end_date timestamp without time zone,
    impact smallint NOT NULL,
    system boolean DEFAULT false NOT NULL
);


ALTER TABLE public.incident OWNER TO pg;

--
-- Name: incident_component_relation; Type: TABLE; Schema: public; Owner: pg
--

CREATE TABLE public.incident_component_relation (
    incident_id integer NOT NULL,
    component_id integer NOT NULL
);


ALTER TABLE public.incident_component_relation OWNER TO pg;

--
-- Name: incident_id_seq; Type: SEQUENCE; Schema: public; Owner: pg
--

CREATE SEQUENCE public.incident_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.incident_id_seq OWNER TO pg;

--
-- Name: incident_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: pg
--

ALTER SEQUENCE public.incident_id_seq OWNED BY public.incident.id;


--
-- Name: incident_status; Type: TABLE; Schema: public; Owner: pg
--

CREATE TABLE public.incident_status (
    id integer NOT NULL,
    incident_id integer,
    "timestamp" timestamp without time zone NOT NULL,
    text character varying NOT NULL,
    status character varying NOT NULL
);


ALTER TABLE public.incident_status OWNER TO pg;

--
-- Name: incident_status_id_seq; Type: SEQUENCE; Schema: public; Owner: pg
--

CREATE SEQUENCE public.incident_status_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.incident_status_id_seq OWNER TO pg;

--
-- Name: incident_status_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: pg
--

ALTER SEQUENCE public.incident_status_id_seq OWNED BY public.incident_status.id;


--
-- Name: component id; Type: DEFAULT; Schema: public; Owner: pg
--

ALTER TABLE ONLY public.component ALTER COLUMN id SET DEFAULT nextval('public.component_id_seq'::regclass);


--
-- Name: component_attribute id; Type: DEFAULT; Schema: public; Owner: pg
--

ALTER TABLE ONLY public.component_attribute ALTER COLUMN id SET DEFAULT nextval('public.component_attribute_id_seq'::regclass);


--
-- Name: incident id; Type: DEFAULT; Schema: public; Owner: pg
--

ALTER TABLE ONLY public.incident ALTER COLUMN id SET DEFAULT nextval('public.incident_id_seq'::regclass);


--
-- Name: incident_status id; Type: DEFAULT; Schema: public; Owner: pg
--

ALTER TABLE ONLY public.incident_status ALTER COLUMN id SET DEFAULT nextval('public.incident_status_id_seq'::regclass);


--
-- Data for Name: component; Type: TABLE DATA; Schema: public; Owner: pg
--

COPY public.component (id, name) FROM stdin;
1	Cloud Container Engine
2	Cloud Container Engine
3	Elastic Cloud Server
4	Elastic Cloud Server
5	Distributed Cache Service
6	Distributed Cache Service
\.


--
-- Data for Name: component_attribute; Type: TABLE DATA; Schema: public; Owner: pg
--

COPY public.component_attribute (id, component_id, name, value) FROM stdin;
1	1	region	EU-DE
2	1	category	Container
3	1	type	cce
4	2	region	EU-NL
5	2	category	Container
6	2	type	cce
7	3	region	EU-DE
8	3	category	Compute
9	3	type	ecs
10	4	region	EU-NL
11	4	category	Compute
12	4	type	ecs
13	5	region	EU-DE
14	5	category	Database
15	5	type	dcs
16	6	region	EU-NL
17	6	category	Database
18	6	type	dcs
\.


--
-- Data for Name: incident; Type: TABLE DATA; Schema: public; Owner: pg
--

COPY public.incident (id, text, start_date, end_date, impact, system) FROM stdin;
1	Closed incident without any update	2024-10-24 10:12:42	2024-10-24 11:12:42	1	f
\.


--
-- Data for Name: incident_component_relation; Type: TABLE DATA; Schema: public; Owner: pg
--

COPY public.incident_component_relation (incident_id, component_id) FROM stdin;
1	1
\.


--
-- Data for Name: incident_status; Type: TABLE DATA; Schema: public; Owner: pg
--

COPY public.incident_status (id, incident_id, "timestamp", text, status) FROM stdin;
1	1	2024-10-24 11:12:42.559346	close incident	resolved
\.


--
-- Name: component_attribute_id_seq; Type: SEQUENCE SET; Schema: public; Owner: pg
--

SELECT pg_catalog.setval('public.component_attribute_id_seq', 18, true);


--
-- Name: component_id_seq; Type: SEQUENCE SET; Schema: public; Owner: pg
--

SELECT pg_catalog.setval('public.component_id_seq', 6, true);


--
-- Name: incident_id_seq; Type: SEQUENCE SET; Schema: public; Owner: pg
--

SELECT pg_catalog.setval('public.incident_id_seq', 1, true);


--
-- Name: incident_status_id_seq; Type: SEQUENCE SET; Schema: public; Owner: pg
--

SELECT pg_catalog.setval('public.incident_status_id_seq', 2, true);


--
-- Name: component_attribute component_attribute_pkey; Type: CONSTRAINT; Schema: public; Owner: pg
--

ALTER TABLE ONLY public.component_attribute
    ADD CONSTRAINT component_attribute_pkey PRIMARY KEY (id);


--
-- Name: component component_pkey; Type: CONSTRAINT; Schema: public; Owner: pg
--

ALTER TABLE ONLY public.component
    ADD CONSTRAINT component_pkey PRIMARY KEY (id);


--
-- Name: incident incident_pkey; Type: CONSTRAINT; Schema: public; Owner: pg
--

ALTER TABLE ONLY public.incident
    ADD CONSTRAINT incident_pkey PRIMARY KEY (id);


--
-- Name: incident_status incident_status_pkey; Type: CONSTRAINT; Schema: public; Owner: pg
--

ALTER TABLE ONLY public.incident_status
    ADD CONSTRAINT incident_status_pkey PRIMARY KEY (id);


--
-- Name: inc_comp_rel; Type: INDEX; Schema: public; Owner: pg
--

CREATE UNIQUE INDEX inc_comp_rel ON public.incident_component_relation USING btree (incident_id, component_id);


--
-- Name: ix_component_attribute_component_id; Type: INDEX; Schema: public; Owner: pg
--

CREATE INDEX ix_component_attribute_component_id ON public.component_attribute USING btree (component_id);


--
-- Name: ix_component_attribute_id; Type: INDEX; Schema: public; Owner: pg
--

CREATE INDEX ix_component_attribute_id ON public.component_attribute USING btree (id);


--
-- Name: ix_component_id; Type: INDEX; Schema: public; Owner: pg
--

CREATE INDEX ix_component_id ON public.component USING btree (id);


--
-- Name: ix_incident_id; Type: INDEX; Schema: public; Owner: pg
--

CREATE INDEX ix_incident_id ON public.incident USING btree (id);


--
-- Name: ix_incident_status_id; Type: INDEX; Schema: public; Owner: pg
--

CREATE INDEX ix_incident_status_id ON public.incident_status USING btree (id);


--
-- Name: ix_incident_status_incident_id; Type: INDEX; Schema: public; Owner: pg
--

CREATE INDEX ix_incident_status_incident_id ON public.incident_status USING btree (incident_id);


--
-- Name: component_attribute component_attribute_component_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: pg
--

ALTER TABLE ONLY public.component_attribute
    ADD CONSTRAINT component_attribute_component_id_fkey FOREIGN KEY (component_id) REFERENCES public.component(id);


--
-- Name: incident_component_relation incident_component_relation_component_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: pg
--

ALTER TABLE ONLY public.incident_component_relation
    ADD CONSTRAINT incident_component_relation_component_id_fkey FOREIGN KEY (component_id) REFERENCES public.component(id);


--
-- PostgreSQL database dump complete
--

