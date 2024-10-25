--
-- PostgreSQL database dump
--

-- Dumped from database version 15.2 (Debian 15.2-1.pgdg110+1)
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
-- Name: incidentimpactenum; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.incidentimpactenum AS ENUM (
    'maintenance',
    'minor',
    'major',
    'outage'
);


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: alembic_version; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.alembic_version (
    version_num character varying(32) NOT NULL
);


--
-- Name: component; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.component (
    id integer NOT NULL,
    name character varying NOT NULL
);


--
-- Name: component_attribute; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.component_attribute (
    id integer NOT NULL,
    component_id integer,
    name character varying NOT NULL,
    value character varying NOT NULL
);


--
-- Name: component_attribute_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.component_attribute_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: component_attribute_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.component_attribute_id_seq OWNED BY public.component_attribute.id;


--
-- Name: component_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.component_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: component_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.component_id_seq OWNED BY public.component.id;


--
-- Name: incident; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.incident (
    id integer NOT NULL,
    text character varying NOT NULL,
    start_date timestamp without time zone NOT NULL,
    end_date timestamp without time zone,
    impact smallint NOT NULL,
    system boolean DEFAULT false NOT NULL
);


--
-- Name: incident_component_relation; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.incident_component_relation (
    incident_id integer NOT NULL,
    component_id integer NOT NULL
);


--
-- Name: incident_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.incident_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: incident_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.incident_id_seq OWNED BY public.incident.id;


--
-- Name: incident_status; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.incident_status (
    id integer NOT NULL,
    incident_id integer,
    "timestamp" timestamp without time zone NOT NULL,
    text character varying NOT NULL,
    status character varying NOT NULL
);


--
-- Name: incident_status_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.incident_status_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: incident_status_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.incident_status_id_seq OWNED BY public.incident_status.id;


--
-- Name: component id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.component ALTER COLUMN id SET DEFAULT nextval('public.component_id_seq'::regclass);


--
-- Name: component_attribute id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.component_attribute ALTER COLUMN id SET DEFAULT nextval('public.component_attribute_id_seq'::regclass);


--
-- Name: incident id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.incident ALTER COLUMN id SET DEFAULT nextval('public.incident_id_seq'::regclass);


--
-- Name: incident_status id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.incident_status ALTER COLUMN id SET DEFAULT nextval('public.incident_status_id_seq'::regclass);


--
-- Data for Name: alembic_version; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.alembic_version (version_num) VALUES
('14621c95e3ee');


--
-- Data for Name: component; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.component (id, name) VALUES
(254, 'DataArts Studio'),
(285, 'Scalable File Service'),
(284, 'Scalable File Service'),
(286, 'Enterprise Dashboard'),
(255, 'Data Lake Insight'),
(264, 'Direct Connect'),
(265, 'Direct Connect'),
(266, 'Domain Name Service'),
(267, 'Domain Name Service'),
(268, 'Elastic IP'),
(269, 'Elastic IP'),
(270, 'Elastic Load Balancing'),
(271, 'Elastic Load Balancing'),
(272, 'NAT Gateway'),
(154, 'Anti DDoS'),
(155, 'Anti DDoS'),
(287, 'Cloud Container Engine'),
(288, 'Cloud Container Engine'),
(158, 'Auto Scaling'),
(159, 'Auto Scaling'),
(160, 'Bare Metal Server'),
(162, 'Cloud Backup and Recovery'),
(163, 'Cloud Backup and Recovery'),
(164, 'Cloud Container Service'),
(165, 'Cloud Container Service'),
(166, 'Cloud Eye'),
(167, 'Cloud Eye'),
(168, 'Cloud Server Backup Service'),
(170, 'Cloud Search Service'),
(171, 'Cloud Search Service'),
(172, 'Cloud Trace Service'),
(173, 'Cloud Trace Service'),
(176, 'Distributed Cache Service'),
(177, 'Distributed Cache Service'),
(178, 'Document Database Service'),
(179, 'Document Database Service'),
(180, 'Dedicated Host'),
(181, 'Dedicated Host'),
(182, 'Data Ingestion Service'),
(184, 'Distributed Message Service'),
(185, 'Distributed Message Service'),
(190, 'Data Warehouse Service'),
(192, 'Elastic Cloud Server'),
(193, 'Elastic Cloud Server'),
(198, 'Elastic Volume Service'),
(199, 'Elastic Volume Service'),
(204, 'Identity and Access Management'),
(205, 'Identity and Access Management'),
(206, 'Image Management Service'),
(207, 'Image Management Service'),
(208, 'Key Management Service'),
(209, 'Key Management Service'),
(210, 'Log Tank Service'),
(211, 'Log Tank Service'),
(212, 'ModelArts'),
(214, 'Map Reduce Service'),
(215, 'Map Reduce Service'),
(218, 'Object Storage Service'),
(219, 'Object Storage Service'),
(224, 'Relational Database Service'),
(225, 'Relational Database Service'),
(228, 'Resource Template Service'),
(229, 'Resource Template Service'),
(230, 'Storage Disaster Recovery Service'),
(231, 'Storage Disaster Recovery Service'),
(234, 'Simple Message Notification'),
(235, 'Simple Message Notification'),
(238, 'Software Repository for Containers'),
(239, 'Software Repository for Containers'),
(242, 'Volume Backup Service'),
(250, 'Web Application Firewall'),
(251, 'Web Application Firewall'),
(252, 'Dedicated Web Application Firewall'),
(253, 'Dedicated Web Application Firewall'),
(273, 'NAT Gateway'),
(274, 'Private Link Access Service'),
(278, 'Virtual Private Cloud'),
(279, 'Virtual Private Cloud'),
(280, 'VPC Endpoint'),
(281, 'VPC Endpoint'),
(282, 'Virtual Private Network'),
(283, 'Virtual Private Network');


--
-- Data for Name: component_attribute; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.component_attribute (id, component_id, name, value) VALUES
(760, 254, 'category', 'Big Data and Data Analysis'),
(761, 254, 'region', 'EU-DE'),
(762, 254, 'type', 'dataarts'),
(850, 284, 'category', 'Storage'),
(851, 284, 'region', 'EU-DE'),
(852, 284, 'type', 'sfs'),
(853, 285, 'category', 'Storage'),
(854, 285, 'region', 'EU-NL'),
(855, 285, 'type', 'sfs'),
(856, 286, 'category', 'Management & Deployment'),
(857, 286, 'region', 'EU-DE'),
(858, 286, 'type', 'enterprise-dashboard'),
(763, 255, 'category', 'Big Data and Data Analysis'),
(764, 255, 'region', 'EU-DE'),
(765, 255, 'type', 'dli'),
(859, 287, 'category', 'Container'),
(860, 287, 'region', 'EU-DE'),
(861, 287, 'type', 'cce'),
(862, 288, 'category', 'Container'),
(863, 288, 'region', 'EU-NL'),
(864, 288, 'type', 'cce'),
(460, 154, 'category', 'Security Services'),
(461, 154, 'region', 'EU-DE'),
(462, 154, 'type', 'antiddos'),
(463, 155, 'category', 'Security Services'),
(464, 155, 'region', 'EU-NL'),
(465, 155, 'type', 'antiddos'),
(472, 158, 'category', 'Compute'),
(473, 158, 'region', 'EU-DE'),
(474, 158, 'type', 'as'),
(475, 159, 'category', 'Compute'),
(476, 159, 'region', 'EU-NL'),
(477, 159, 'type', 'as'),
(478, 160, 'category', 'Compute'),
(479, 160, 'region', 'EU-DE'),
(480, 160, 'type', 'bms'),
(484, 162, 'category', 'Storage'),
(485, 162, 'region', 'EU-DE'),
(486, 162, 'type', 'cbr'),
(487, 163, 'category', 'Storage'),
(488, 163, 'region', 'EU-NL'),
(489, 163, 'type', 'cbr'),
(496, 166, 'category', 'Management & Deployment'),
(497, 166, 'region', 'EU-DE'),
(498, 166, 'type', 'ces'),
(499, 167, 'category', 'Management & Deployment'),
(500, 167, 'region', 'EU-NL'),
(501, 167, 'type', 'ces'),
(502, 168, 'category', 'Storage'),
(503, 168, 'region', 'EU-DE'),
(504, 168, 'type', 'csbs'),
(508, 170, 'category', 'Big Data and Data Analysis'),
(509, 170, 'region', 'EU-DE'),
(510, 170, 'type', 'css'),
(511, 171, 'category', 'Big Data and Data Analysis'),
(512, 171, 'region', 'EU-NL'),
(513, 171, 'type', 'css'),
(514, 172, 'category', 'Management & Deployment'),
(515, 172, 'region', 'EU-DE'),
(516, 172, 'type', 'cts'),
(517, 173, 'category', 'Management & Deployment'),
(518, 173, 'region', 'EU-NL'),
(519, 173, 'type', 'cts'),
(526, 176, 'category', 'Database'),
(527, 176, 'region', 'EU-DE'),
(528, 176, 'type', 'dcs'),
(529, 177, 'category', 'Database'),
(530, 177, 'region', 'EU-NL'),
(531, 177, 'type', 'dcs'),
(532, 178, 'category', 'Database'),
(533, 178, 'region', 'EU-DE'),
(534, 178, 'type', 'dds'),
(535, 179, 'category', 'Database'),
(536, 179, 'region', 'EU-NL'),
(537, 179, 'type', 'dds'),
(538, 180, 'category', 'Compute'),
(539, 180, 'region', 'EU-DE'),
(540, 180, 'type', 'deh'),
(541, 181, 'category', 'Compute'),
(542, 181, 'region', 'EU-NL'),
(543, 181, 'type', 'deh'),
(544, 182, 'category', 'Big Data and Data Analysis'),
(545, 182, 'region', 'EU-DE'),
(546, 182, 'type', 'dis'),
(550, 184, 'category', 'Application Services'),
(551, 184, 'region', 'EU-DE'),
(552, 184, 'type', 'dms'),
(553, 185, 'category', 'Application Services'),
(554, 185, 'region', 'EU-NL'),
(555, 185, 'type', 'dms'),
(568, 190, 'category', 'Big Data and Data Analysis'),
(569, 190, 'region', 'EU-DE'),
(570, 190, 'type', 'dws'),
(574, 192, 'category', 'Compute'),
(575, 192, 'region', 'EU-DE'),
(576, 192, 'type', 'ecs'),
(577, 193, 'category', 'Compute'),
(578, 193, 'region', 'EU-NL'),
(579, 193, 'type', 'ecs'),
(592, 198, 'category', 'Storage'),
(593, 198, 'region', 'EU-DE'),
(594, 198, 'type', 'evs'),
(595, 199, 'category', 'Storage'),
(596, 199, 'region', 'EU-NL'),
(597, 199, 'type', 'evs'),
(610, 204, 'category', 'Security Services'),
(611, 204, 'region', 'EU-DE'),
(612, 204, 'type', 'iam'),
(613, 205, 'category', 'Security Services'),
(614, 205, 'region', 'EU-NL'),
(615, 205, 'type', 'iam'),
(616, 206, 'category', 'Compute'),
(617, 206, 'region', 'EU-DE'),
(618, 206, 'type', 'ims'),
(619, 207, 'category', 'Compute'),
(620, 207, 'region', 'EU-NL'),
(621, 207, 'type', 'ims'),
(622, 208, 'category', 'Security Services'),
(623, 208, 'region', 'EU-DE'),
(624, 208, 'type', 'kms'),
(625, 209, 'category', 'Security Services'),
(626, 209, 'region', 'EU-NL'),
(627, 209, 'type', 'kms'),
(628, 210, 'category', 'Management & Deployment'),
(629, 210, 'region', 'EU-DE'),
(630, 210, 'type', 'lts'),
(631, 211, 'category', 'Management & Deployment'),
(632, 211, 'region', 'EU-NL'),
(633, 211, 'type', 'lts'),
(634, 212, 'category', 'Big Data and Data Analysis'),
(635, 212, 'region', 'EU-DE'),
(636, 212, 'type', 'ma'),
(640, 214, 'category', 'Big Data and Data Analysis'),
(641, 214, 'region', 'EU-DE'),
(642, 214, 'type', 'mrs'),
(643, 215, 'category', 'Big Data and Data Analysis'),
(644, 215, 'region', 'EU-NL'),
(645, 215, 'type', 'mrs'),
(652, 218, 'category', 'Storage'),
(653, 218, 'region', 'EU-DE'),
(654, 218, 'type', 'obs'),
(655, 219, 'category', 'Storage'),
(656, 219, 'region', 'EU-NL'),
(657, 219, 'type', 'obs'),
(670, 224, 'category', 'Database'),
(671, 224, 'region', 'EU-DE'),
(672, 224, 'type', 'rds'),
(673, 225, 'category', 'Database'),
(674, 225, 'region', 'EU-NL'),
(675, 225, 'type', 'rds'),
(682, 228, 'category', 'Management & Deployment'),
(684, 228, 'type', 'rts'),
(685, 229, 'category', 'Management & Deployment'),
(687, 229, 'type', 'rts'),
(688, 230, 'category', 'Storage'),
(689, 230, 'region', 'EU-DE'),
(690, 230, 'type', 'sdrs'),
(691, 231, 'category', 'Storage'),
(692, 231, 'region', 'EU-NL'),
(693, 231, 'type', 'sdrs'),
(700, 234, 'category', 'Application Services'),
(701, 234, 'region', 'EU-DE'),
(702, 234, 'type', 'smn'),
(703, 235, 'category', 'Application Services'),
(704, 235, 'region', 'EU-NL'),
(705, 235, 'type', 'smn'),
(712, 238, 'category', 'Container'),
(713, 238, 'region', 'EU-DE'),
(714, 238, 'type', 'swr'),
(715, 239, 'category', 'Container'),
(716, 239, 'region', 'EU-NL'),
(717, 239, 'type', 'swr'),
(724, 242, 'category', 'Storage'),
(725, 242, 'region', 'EU-DE'),
(726, 242, 'type', 'vbs'),
(748, 250, 'category', 'Security Services'),
(749, 250, 'region', 'EU-DE'),
(750, 250, 'type', 'waf'),
(751, 251, 'category', 'Security Services'),
(752, 251, 'region', 'EU-NL'),
(753, 251, 'type', 'waf'),
(754, 252, 'category', 'Security Services'),
(755, 252, 'region', 'EU-DE'),
(756, 252, 'type', 'wafd'),
(757, 253, 'category', 'Security Services'),
(758, 253, 'region', 'EU-NL'),
(759, 253, 'type', 'wafd'),
(790, 264, 'category', 'Network'),
(791, 264, 'region', 'EU-DE'),
(792, 264, 'type', 'dc'),
(793, 265, 'category', 'Network'),
(794, 265, 'region', 'EU-NL'),
(795, 265, 'type', 'dc'),
(796, 266, 'category', 'Network'),
(797, 266, 'region', 'EU-DE'),
(798, 266, 'type', 'dns'),
(799, 267, 'category', 'Network'),
(800, 267, 'region', 'EU-NL'),
(801, 267, 'type', 'dns'),
(802, 268, 'category', 'Network'),
(803, 268, 'region', 'EU-DE'),
(804, 268, 'type', 'eip'),
(805, 269, 'category', 'Network'),
(806, 269, 'region', 'EU-NL'),
(807, 269, 'type', 'eip'),
(808, 270, 'category', 'Network'),
(809, 270, 'region', 'EU-DE'),
(810, 270, 'type', 'elb'),
(811, 271, 'category', 'Network'),
(812, 271, 'region', 'EU-NL'),
(813, 271, 'type', 'elb'),
(814, 272, 'category', 'Network'),
(815, 272, 'region', 'EU-DE'),
(816, 272, 'type', 'natgw'),
(817, 273, 'category', 'Network'),
(818, 273, 'region', 'EU-NL'),
(819, 273, 'type', 'natgw'),
(820, 274, 'category', 'Network'),
(821, 274, 'region', 'EU-DE'),
(822, 274, 'type', 'plas'),
(832, 278, 'category', 'Network'),
(833, 278, 'region', 'EU-DE'),
(834, 278, 'type', 'vpc'),
(835, 279, 'category', 'Network'),
(836, 279, 'region', 'EU-NL'),
(837, 279, 'type', 'vpc'),
(838, 280, 'category', 'Network'),
(839, 280, 'region', 'EU-DE'),
(840, 280, 'type', 'vpcep'),
(841, 281, 'category', 'Network'),
(842, 281, 'region', 'EU-NL'),
(843, 281, 'type', 'vpcep'),
(844, 282, 'category', 'Network'),
(845, 282, 'region', 'EU-DE'),
(846, 282, 'type', 'vpn'),
(847, 283, 'category', 'Network'),
(848, 283, 'region', 'EU-NL'),
(849, 283, 'type', 'vpn');


--
-- Data for Name: incident; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.incident (id, text, start_date, end_date, impact, system) VALUES
(95, 'Incident', '2023-12-12 07:13:15.267751', '2023-12-12 11:26:19.64391', 1, false),
(64, 'ECS outage', '2023-11-08 23:00:00', '2023-11-08 23:13:00', 3, false),
(97, 'Upgrade of the backend systems. During the timeframe, you may recognize a higher latency for a very short time, but no downtime.', '2023-12-12 12:00:00', '2023-12-12 17:30:00', 0, false),
(65, 'EVS outage', '2023-11-07 11:00:00', '2023-11-07 11:17:00', 3, false),
(66, 'OBS outage', '2023-11-06 16:00:00', '2023-11-06 17:09:00', 3, false),
(96, 'Upgrade of backend systems. During the timeframe, you may recognize a higher latency for a short time, but no downtime', '2023-12-12 12:00:00', '2023-12-12 11:59:27.067541', 0, false),
(67, 'OBS outage', '2023-10-11 05:00:00', '2023-10-11 05:04:32', 3, false),
(98, 'Upgrade of backend systems. During the timeframe, you may recognize a higher latency for a very short time, but no downtime. In case of issues, please contact our Service Desk.', '2023-12-12 12:35:00', '2023-12-12 15:53:31.262114', 0, false),
(99, 'Incident', '2023-12-12 16:17:02.681289', '2023-12-12 17:02:03.687942', 1, false),
(100, 'Incident', '2023-12-12 20:31:00.161432', '2023-12-12 21:53:05.754742', 1, false),
(101, 'Incident', '2023-12-13 06:55:19.661677', '2023-12-13 08:44:36.203924', 1, false),
(102, 'Incident', '2023-12-13 18:25:11.025512', '2023-12-13 18:56:52.957866', 1, false),
(103, 'Interruptions may occur in the mentioned timeframe due to planned upgrades.', '2023-12-14 11:00:00', '2023-12-14 19:30:00', 0, false),
(104, 'Incident', '2023-12-14 10:33:16.430113', '2023-12-14 11:35:58.590019', 1, false),
(105, 'Incident', '2023-12-14 17:19:11.494477', '2023-12-14 22:47:04.538872', 1, false),
(106, 'Incident', '2023-12-15 00:40:07.05433', '2023-12-15 08:27:37.203292', 1, false),
(107, 'Incident', '2023-12-16 03:52:03.228579', '2023-12-16 09:58:49.300229', 1, false),
(108, 'Incident', '2023-12-16 16:07:47.172504', '2023-12-16 22:17:44.609537', 1, false),
(109, 'Incident', '2023-12-17 02:37:49.001971', '2023-12-17 19:55:03.493711', 1, false),
(172, 'Maintenance On Database Services', '2024-05-15 09:00:00', '2024-05-21 16:00:00', 0, false),
(82, 'Preparation for IPv6', '2023-12-01 09:00:00', '2023-12-20 22:00:00', 0, false),
(80, 'Incident', '2023-12-07 19:57:11.873768', '2023-12-08 10:56:56.524241', 1, false),
(83, 'Incident', '2023-12-08 11:06:55.875457', '2023-12-08 13:35:36.500178', 1, false),
(84, 'Incident', '2023-12-08 12:48:42.779288', '2023-12-08 15:49:55.681422', 2, false),
(85, 'Incident', '2023-12-08 14:46:34.623115', '2023-12-08 15:50:20.672657', 1, false),
(87, 'Incident (ModelArts)', '2023-12-08 19:49:02.420411', '2023-12-09 08:38:50.274193', 1, false),
(88, 'Incident', '2023-12-09 13:55:39.582482', '2023-12-09 22:42:23.484453', 2, false),
(89, 'Incident (Image Management Service)', '2023-12-08 19:49:02.420411', '2023-12-09 22:43:10.662423', 1, false),
(90, 'Incident', '2023-12-10 16:42:07.171694', '2023-12-10 20:45:58.859011', 2, false),
(86, 'Incident', '2023-12-08 19:49:02.420411', '2023-12-10 20:46:12.118361', 1, false),
(91, 'Incident', '2023-12-11 01:46:43.646355', '2023-12-11 07:19:59.95186', 2, false),
(92, 'Incident', '2023-12-11 01:46:46.250426', '2023-12-11 07:20:11.898049', 1, false),
(93, 'Incident', '2023-12-11 12:35:36.4661', '2023-12-11 13:53:46.98246', 1, false),
(94, 'Incident', '2023-12-11 16:53:17.54253', '2023-12-11 21:43:43.933838', 1, false),
(110, 'VPN service will be updated. Short interruptions of VPN sessions may appear.', '2024-01-04 19:00:00', '2024-01-04 23:59:00', 0, false),
(111, 'Please be informed that there is a maintenance with the Object Storage service "OBS".', '2024-01-12 10:00:00', '2024-01-12 18:00:00', 0, false),
(112, 'Incident', '2024-01-16 18:38:01.477127', '2024-01-18 17:27:59.855478', 1, false),
(113, 'Incident', '2024-01-20 21:48:23.552756', '2024-01-21 10:27:37.33587', 1, false),
(115, 'Impact can occur on persistent connection, which do not have a re-connection mechanism like SSH. If your connection gets disconnected, a manually intervention to reestablish a connection is needed.', '2024-01-25 17:00:00', '2024-01-25 23:00:00', 0, false),
(134, 'Regular update of OBS. No customer impact or interruption to be expected.', '2024-03-08 12:00:00', '2024-03-08 18:00:00', 0, false),
(114, 'Incident', '2024-01-22 08:10:02.035346', '2024-01-24 09:10:53.041768', 1, false),
(117, 'Hotfix installation to improve single logout process. During the operation, API and UI interactions (logging in, using SSO, trying to fetch new token) could be affected for about 5-10 minutes.', '2024-01-25 16:00:00', '2024-01-25 17:00:00', 0, false),
(118, 'Impact can occur on persistent connection, which do not have a re-connection mechanism like SSH. If your connection gets disconnected, a manually intervention to reestablish a connection is needed.', '2024-01-26 19:30:00', '2024-01-26 22:30:00', 0, false),
(119, 'Impact can occur on persistent connection, which do not have a re-connection mechanism like SSH. If your connection gets disconnected, a manually intervention to reestablish a connection is needed.', '2024-01-27 08:00:00', '2024-01-27 14:00:00', 0, false),
(120, 'Impact can occur on persistent connection, which do not have a re-connection mechanism like SSH. If your connection gets disconnected, a manually intervention to reestablish a connection is needed.', '2024-01-28 19:30:00', '2024-01-28 22:30:00', 0, false),
(130, 'Incident', '2024-02-05 15:18:44.30385', '2024-02-05 16:41:17.322072', 1, false),
(116, 'Incident', '2024-01-24 10:11:44.342655', '2024-01-26 08:57:12.920872', 1, false),
(121, 'Incident', '2024-01-27 11:10:54.657531', '2024-01-30 15:32:04.245169', 1, false),
(122, 'Impact can occur on persistent connection, which do not have a re-connection mechanism like SSH. If your connection gets disconnected, a manually intervention to reestablish a connection is needed.', '2024-01-30 18:00:00', '2024-01-30 23:00:00', 0, false),
(123, 'CBR console may show some error during the maintenance. No downtime is expected as part of a normal operation.', '2024-01-31 08:30:00', '2024-01-31 19:30:00', 0, false),
(124, 'Partial performance degradation may occur in AZ3 during the maintenance. No downtime to be expected.', '2024-01-31 11:00:00', '2024-01-31 20:00:00', 0, false),
(125, 'Impact can occur on persistent connection, which do not have a re-connection mechanism like SSH. If your connection gets disconnected, a manually intervention to reestablish a connection is needed.', '2024-02-01 19:30:00', '2024-02-01 23:30:00', 0, false),
(126, 'Impact can occur on persistent connection, which do not have a re-connection mechanism like SSH. If your connection gets disconnected, a manually intervention to reestablish a connection is needed.', '2024-02-01 19:30:00', '2024-02-02 00:30:00', 0, false),
(127, 'Impact can occur on persistent connection, which do not have a re-connection mechanism like SSH. If your connection gets disconnected, a manually intervention to reestablish a connection is needed.', '2024-02-03 08:00:00', '2024-02-03 12:00:00', 0, false),
(128, 'Impact can occur on persistent connection, which do not have a re-connection mechanism like SSH. If your connection gets disconnected, a manually intervention to reestablish a connection is needed.', '2024-02-02 19:30:00', '2024-02-03 00:30:00', 0, false),
(131, 'Incident', '2024-02-08 22:30:04.609835', '2024-02-09 07:19:50.1731', 1, false),
(129, 'Incident', '2024-02-02 21:56:37.445125', '2024-02-05 07:45:43.710479', 1, false),
(132, 'Incident', '2024-03-01 00:34:26.973699', '2024-03-01 08:15:39.364993', 2, false),
(133, 'In SWR console, Create new repository, upload image will not work. But download image will work. In CLI, Push any new image or new tag of existing image will not work. But pulling image will work.', '2024-03-07 19:00:00', '2024-03-07 21:00:00', 0, false),
(135, 'Incident', '2024-03-11 07:21:25.70358', '2024-03-11 09:20:00.633587', 2, false),
(136, 'Incident', '2024-03-13 15:23:11.706296', '2024-03-13 21:07:33.465079', 2, false),
(137, 'Incident', '2024-03-18 12:24:37.446294', '2024-03-18 12:44:29.819081', 2, false),
(138, 'Incident', '2024-03-19 15:28:44.633254', '2024-03-19 16:36:01.136359', 2, false),
(139, 'Incident', '2024-03-19 16:56:29.603', '2024-03-19 19:51:23.838938', 2, false),
(140, 'In SWR console, create new repository or upload image will not work, but download image will work. In CLI, push any new image or new tag of existing image will not work, but Pulling image will work.', '2024-03-21 20:00:00', '2024-03-21 22:00:00', 0, false),
(141, 'Incident', '2024-03-21 22:07:41.731244', '2024-03-22 07:23:15.592219', 2, false),
(142, 'Incident', '2024-03-24 04:09:38.147167', '2024-03-24 07:22:04.491162', 2, false),
(143, 'Incident', '2024-03-25 17:49:31.244906', '2024-03-25 20:53:05.170992', 1, false),
(173, 'Scheduled maintenance on CBR, CSBS and VBS service.', '2024-05-16 06:00:00', '2024-05-16 16:00:00', 0, false),
(144, 'Incident', '2024-03-26 07:34:12.23871', '2024-03-26 08:38:05.24078', 1, false),
(145, 'Incident', '2024-03-26 12:36:32.516935', '2024-03-26 12:59:05.120533', 1, false),
(147, 'Incident', '2024-04-03 22:17:46.277524', '2024-04-04 07:47:39.450131', 2, false),
(174, 'Maintenance on Elastic Load Balancer', '2024-05-20 15:30:00', '2024-05-20 19:30:00', 0, false),
(148, 'Incident', '2024-04-04 15:52:15.942181', '2024-04-04 16:56:06.796429', 2, false),
(175, 'Maintenance on Elastic Load Balancer', '2024-05-21 15:30:00', '2024-05-21 19:30:00', 0, false),
(149, 'Incident', '2024-04-04 21:17:55.285935', '2024-04-05 05:15:52.735537', 2, false),
(176, 'Maintenance on Elastic Load Balancer', '2024-05-22 15:30:00', '2024-05-22 19:30:00', 0, false),
(150, 'Incident', '2024-04-05 06:07:49.584639', '2024-04-05 07:44:14.2772', 2, false),
(151, 'Incident', '2024-04-05 12:03:17.382509', '2024-04-05 12:31:26.334489', 2, false),
(146, 'Incident', '2024-04-03 07:12:21.953994', '2024-04-05 12:35:30.605498', 1, false),
(152, 'Incident', '2024-04-05 12:38:02.6132', '2024-04-09 06:28:48.46574', 2, false),
(154, 'No downtime or service interruption is expected', '2024-04-10 14:00:00', '2024-04-11 00:00:00', 0, false),
(153, 'Incident', '2024-04-09 13:03:38.927385', '2024-04-11 09:06:13.705123', 2, false),
(155, 'Incident', '2024-04-11 10:03:30.333618', '2024-04-11 12:37:18.003512', 2, false),
(177, 'Maintenance on Elastic Load Balancer', '2024-05-23 15:30:00', '2024-05-23 19:30:00', 0, false),
(157, 'Potential minor service interruption', '2024-04-14 22:00:00', '2024-04-16 04:00:00', 0, false),
(159, 'OBS Upgrade', '2024-04-16 08:00:00', '2024-04-16 16:00:00', 0, false),
(158, 'Incident', '2024-04-12 19:08:37.772218', '2024-04-16 14:51:49.387971', 2, false),
(156, 'Cloud Eye Service: representation of health status incorrect', '2024-04-11 21:37:29.241098', '2024-04-16 14:53:04.659151', 1, false),
(161, 'Potential minor service interruption', '2024-04-18 06:00:00', '2024-04-17 22:00:00', 0, false),
(160, 'Scheduled maintenance of CCE (Cloud Container Engine)', '2024-04-19 17:00:00', '2024-04-19 19:00:00', 0, false),
(178, 'Incident', '2024-05-18 06:03:11.93376', '2024-05-18 07:42:25.286452', 2, false),
(162, 'Incident', '2024-04-18 21:44:28.631415', '2024-04-19 04:18:37.609441', 2, false),
(163, 'Incident', '2024-04-25 15:23:13.446296', '2024-04-25 16:21:04.334983', 2, false),
(164, 'Incident', '2024-04-28 15:52:04.043663', '2024-04-29 06:02:02.473311', 1, false),
(165, 'Scheduled maintenance on KMS service', '2024-05-02 08:30:00', '2024-05-02 14:30:00', 0, false),
(180, 'Maintenance on Elastic Load Balancer', '2024-05-23 15:30:00', '2024-05-23 19:30:00', 0, false),
(167, 'RDS maintenance', '2024-05-06 07:00:00', '2024-05-10 15:00:00', 0, false),
(179, 'Maintenance on Elastic Load Balancer', '2024-05-22 13:30:00', '2024-05-22 13:42:19.030745', 0, false),
(168, 'Incident', '2024-05-06 12:02:16.635159', '2024-05-06 13:47:16.103093', 1, false),
(169, 'Incident', '2024-05-07 08:16:50.703635', '2024-05-07 10:51:28.416606', 2, false),
(170, 'Scheduled maintenance on CBR service. No downtime is expected as part of a normal operation.', '2024-05-15 06:00:00', '2024-05-15 16:00:00', 0, false),
(171, 'Incident', '2024-05-14 08:47:00.633653', '2024-05-14 15:03:38.842885', 1, false),
(210, 'Incident', '2024-07-23 09:04:25.62749', '2024-07-23 09:41:07.930458', 2, false),
(181, 'Incident', '2024-05-26 00:07:40.429323', '2024-05-26 06:33:27.17926', 1, false),
(182, 'Incident', '2024-05-26 11:21:14.660999', '2024-05-26 14:22:51.217647', 1, false),
(211, 'Incident', '2024-07-23 09:48:14.81952', '2024-07-23 12:01:22.405479', 2, false),
(183, 'Incident', '2024-05-27 00:10:09.652397', '2024-05-27 06:12:40.384869', 1, false),
(185, 'Modify VPC- Cascading Proxy to enable routetable_support_type', '2024-05-30 18:00:00', '2024-05-30 20:00:00', 0, false),
(184, 'Modify VPC- Cascading Proxy to enable routetable_support_type', '2024-05-30 18:00:00', '2024-05-28 09:47:09.998293', 0, false),
(187, 'Incident', '2024-05-31 02:07:29.936947', '2024-05-31 06:26:28.209524', 1, false),
(188, 'Incident', '2024-06-03 14:15:22.072377', '2024-06-04 07:12:19.781809', 2, false),
(189, 'Incident', '2024-06-05 13:04:50.742272', '2024-06-05 14:15:57.011995', 2, false),
(190, 'Incident', '2024-06-06 13:14:07.695586', '2024-06-06 15:13:32.158747', 2, false),
(191, 'Incident', '2024-06-07 07:26:15.884375', '2024-06-10 11:49:38.374428', 2, false),
(192, 'RDS Database Service Maintenance', '2024-06-11 07:00:00', '2024-06-11 10:00:00', 0, false),
(193, 'Scheduled maintenance on VPC Service', '2024-06-17 08:00:00', '2024-06-17 13:30:00', 0, false),
(194, 'Scheduled maintenance on ELB service.', '2024-06-17 08:00:00', '2024-06-17 15:30:00', 0, false),
(195, 'Maintenance on VPC and ELB Services', '2024-06-19 07:00:00', '2024-06-19 16:00:00', 0, false),
(196, 'Scheduled maintenance CCE service', '2024-06-19 11:00:00', '2024-06-19 21:00:00', 0, false),
(197, 'Maintenance on VPC and ELB Services', '2024-06-20 08:00:00', '2024-06-20 16:00:00', 0, false),
(198, 'Maintenance on VPC and ELB Services', '2024-06-21 08:00:00', '2024-06-21 16:00:00', 0, false),
(199, 'Incident', '2024-06-21 16:39:58.012171', '2024-06-24 12:06:37.204832', 2, false),
(186, 'Openstack Upgrade in Region: EU-DE', '2024-05-28 07:00:00', '2024-06-28 07:09:50.302649', 0, false),
(201, 'Incident', '2024-07-05 06:29:33.161095', '2024-07-05 09:45:23.684639', 2, false),
(202, 'Incident', '2024-07-06 06:58:49.972906', '2024-07-08 06:55:27.535162', 2, false),
(203, 'Incident', '2024-07-10 08:37:46.853727', '2024-07-10 08:46:03.729216', 1, false),
(204, 'Scheduled Maintenance: potential slowness of some services for some seconds', '2024-07-11 16:00:00', '2024-07-11 20:00:00', 0, false),
(205, 'Incident', '2024-07-14 12:09:24.645447', '2024-07-14 13:00:31.930957', 1, false),
(206, 'Incident', '2024-07-19 14:38:27.590572', '2024-07-19 15:30:32.468675', 2, false),
(207, 'Incident', '2024-07-21 17:55:52.513963', '2024-07-22 06:03:48.632172', 2, false),
(212, 'GaussDB migration', '2024-07-24 16:00:00', '2024-07-24 06:55:09.075481', 0, false),
(208, 'Incident', '2024-07-22 15:32:28.74657', '2024-07-22 18:07:17.344752', 2, false),
(209, 'Incident', '2024-07-22 18:58:13.882338', '2024-07-23 08:22:12.719467', 2, false),
(213, 'GaussDB migration in EU-DE', '2024-07-25 16:00:00', '2024-07-25 20:00:00', 0, false),
(214, 'IAM Maitenanace', '2024-07-31 16:00:00', '2024-07-31 22:00:00', 0, false),
(200, 'OpenStack Upgrade in regions EU-DE/EU-NL', '2024-06-28 16:00:00', '2024-07-24 13:16:39.479569', 0, false),
(224, 'Incident', '2024-08-01 17:21:05.959751', '2024-08-01 19:31:30.779036', 1, false),
(216, 'Incident', '2024-07-25 20:01:30.832177', '2024-07-25 20:24:51.086716', 2, false),
(215, 'Incident', '2024-07-25 19:22:22.747592', '2024-07-25 20:25:05.931352', 1, false),
(217, 'Incident', '2024-07-26 02:08:37.058781', '2024-07-26 06:12:30.294499', 2, false),
(218, 'Incident', '2024-07-30 04:06:33.293578', '2024-07-30 06:04:38.13671', 2, false),
(219, 'Maintenance on DDS (Document Database Service)', '2024-07-31 19:00:00', '2024-07-31 23:00:00', 0, false),
(220, 'GaussDB Migration', '2024-08-01 15:00:00', '2024-08-01 21:59:00', 0, false),
(221, 'Planned DDS Maintenance', '2024-07-31 06:00:00', '2024-07-31 13:00:00', 0, false),
(222, 'ModelArts Maintenance', '2024-07-31 09:30:00', '2024-08-03 10:59:00', 0, false),
(223, 'Incident', '2024-08-01 10:44:06.105552', '2024-08-01 11:18:51.595428', 2, false),
(225, 'Incident', '2024-08-02 04:33:10.023604', '2024-08-02 05:51:21.865929', 2, false),
(226, 'Incident', '2024-08-02 13:30:55.606788', '2024-08-02 14:07:39.151378', 2, false),
(227, 'Maintenance on RDS service', '2024-08-07 12:30:00', '2024-08-07 13:30:00', 0, false),
(228, 'OBS Traffic re-routing', '2024-08-15 17:00:00', '2024-08-15 20:00:00', 0, false),
(229, 'Incident', '2024-08-12 18:55:14.497247', '2024-08-13 06:08:03.184888', 2, false),
(230, 'Incident', '2024-08-13 00:22:47.32627', '2024-08-13 06:08:41.704226', 1, false),
(231, 'Incident', '2024-08-14 01:44:33.931601', '2024-08-14 06:00:11.575132', 2, false),
(232, 'Incident', '2024-08-14 12:58:45.243829', '2024-08-14 13:54:32.41695', 2, false),
(233, 'OBS Traffic re-routing', '2024-08-22 17:00:00', '2024-08-22 20:00:00', 0, false),
(235, 'Incident', '2024-08-22 00:22:27.977741', '2024-08-22 07:18:40.904869', 1, false),
(236, 'Maintenance on CBR Services', '2024-08-23 07:00:00', '2024-08-23 17:00:00', 0, false),
(234, 'Maintenance on CBR Services', '2024-08-22 07:00:00', '2024-08-22 14:51:09.574122', 0, false),
(237, 'Incident', '2024-08-23 00:19:29.272015', '2024-08-23 07:47:52.148093', 2, false);


--
-- Data for Name: incident_component_relation; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.incident_component_relation (incident_id, component_id) VALUES
(110, 282),
(111, 218),
(112, 173),
(113, 279),
(114, 160),
(114, 182),
(114, 166),
(115, 281),
(64, 192),
(65, 198),
(66, 218),
(67, 218),
(116, 166),
(117, 205),
(117, 204),
(118, 281),
(119, 281),
(120, 281),
(116, 266),
(121, 166),
(121, 158),
(121, 239),
(121, 251),
(122, 281),
(123, 163),
(124, 198),
(125, 280),
(126, 280),
(78, 264),
(80, 264),
(127, 280),
(82, 192),
(80, 285),
(80, 160),
(80, 155),
(80, 192),
(80, 181),
(80, 267),
(80, 184),
(80, 274),
(80, 228),
(80, 242),
(80, 214),
(80, 252),
(80, 230),
(80, 198),
(80, 279),
(80, 163),
(80, 281),
(80, 193),
(80, 225),
(80, 178),
(80, 212),
(128, 280),
(83, 181),
(84, 264),
(84, 228),
(84, 272),
(85, 270),
(84, 282),
(129, 171),
(86, 273),
(87, 212),
(88, 212),
(129, 167),
(89, 207),
(129, 266),
(86, 279),
(90, 212),
(91, 228),
(92, 242),
(92, 230),
(93, 212),
(94, 280),
(94, 264),
(95, 212),
(95, 242),
(95, 252),
(95, 272),
(95, 204),
(95, 176),
(95, 214),
(95, 284),
(95, 230),
(95, 166),
(95, 228),
(95, 206),
(95, 168),
(95, 184),
(95, 208),
(95, 198),
(95, 281),
(95, 279),
(95, 163),
(95, 205),
(95, 167),
(96, 210),
(97, 198),
(98, 198),
(99, 212),
(100, 212),
(101, 280),
(102, 212),
(103, 198),
(104, 212),
(105, 212),
(106, 212),
(107, 173),
(107, 264),
(108, 284),
(109, 264),
(130, 177),
(130, 210),
(131, 173),
(132, 271),
(133, 238),
(134, 218),
(135, 254),
(135, 172),
(136, 173),
(137, 254),
(138, 172),
(139, 254),
(139, 172),
(140, 238),
(141, 172),
(142, 254),
(143, 192),
(143, 177),
(143, 179),
(143, 283),
(143, 235),
(143, 167),
(143, 273),
(143, 163),
(143, 159),
(143, 209),
(143, 199),
(144, 224),
(144, 234),
(144, 192),
(144, 177),
(144, 179),
(144, 283),
(144, 235),
(144, 167),
(144, 273),
(144, 163),
(144, 159),
(144, 209),
(144, 199),
(145, 250),
(146, 255),
(146, 224),
(146, 234),
(146, 242),
(146, 192),
(146, 265),
(146, 179),
(146, 283),
(146, 235),
(146, 167),
(146, 273),
(146, 163),
(146, 159),
(146, 199),
(147, 166),
(148, 166),
(149, 166),
(150, 166),
(151, 166),
(152, 166),
(153, 166),
(154, 204),
(154, 205),
(155, 166),
(156, 166),
(157, 278),
(158, 166),
(159, 219),
(161, 278),
(162, 273),
(163, 192),
(164, 173),
(165, 209),
(165, 208),
(167, 224),
(167, 225),
(168, 184),
(169, 173),
(170, 163),
(171, 173),
(172, 179),
(172, 178),
(173, 242),
(173, 168),
(173, 162),
(174, 270),
(175, 192),
(176, 270),
(177, 270),
(178, 228),
(179, 270),
(180, 270),
(181, 173),
(182, 173),
(183, 173),
(184, 280),
(184, 192),
(185, 192),
(185, 278),
(186, 218),
(187, 173),
(188, 173),
(189, 173),
(190, 171),
(191, 173),
(192, 224),
(192, 225),
(193, 278),
(194, 270),
(195, 281),
(160, 287),
(197, 279),
(197, 271),
(198, 279),
(198, 271),
(199, 173),
(200, 218),
(201, 185),
(202, 173),
(203, 209),
(204, 158),
(204, 198),
(204, 192),
(204, 278),
(204, 206),
(204, 160),
(205, 180),
(206, 173),
(207, 173),
(208, 198),
(209, 198),
(210, 198),
(211, 198),
(212, 158),
(212, 192),
(212, 206),
(212, 278),
(212, 160),
(212, 198),
(213, 198),
(213, 158),
(213, 206),
(213, 278),
(213, 192),
(213, 160),
(214, 207),
(214, 181),
(214, 251),
(214, 264),
(214, 184),
(214, 250),
(214, 185),
(214, 255),
(214, 192),
(214, 177),
(214, 172),
(214, 265),
(214, 210),
(214, 198),
(214, 170),
(214, 218),
(214, 171),
(214, 219),
(214, 180),
(214, 285),
(214, 211),
(214, 173),
(214, 206),
(214, 176),
(214, 193),
(214, 199),
(214, 190),
(214, 284),
(216, 159),
(216, 199),
(215, 181),
(216, 173),
(215, 211),
(217, 173),
(218, 173),
(219, 178),
(219, 179),
(220, 158),
(220, 192),
(220, 278),
(220, 206),
(220, 160),
(220, 198),
(221, 179),
(221, 178),
(222, 212),
(223, 180),
(224, 180),
(225, 173),
(226, 173),
(196, 287),
(214, 287),
(214, 288),
(227, 224),
(228, 218),
(229, 173),
(230, 160),
(231, 173),
(232, 173),
(233, 218),
(234, 163),
(235, 160),
(236, 163),
(237, 160);


--
-- Data for Name: incident_status; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.incident_status (id, incident_id, "timestamp", text, status) VALUES
(97, 64, '2023-12-07 16:07:55.559346', 'Outage ended', 'resolved'),
(98, 65, '2023-12-07 16:13:31.619079', 'Outage completed', 'resolved'),
(99, 66, '2023-12-07 16:17:42.451396', 'Outage completed', 'resolved'),
(100, 67, '2023-12-07 16:23:27.371031', 'Outage completed', 'resolved'),
(121, 80, '2023-12-08 10:56:56.516827', 'Minor issues due to the introduction of new Status Dashboard.', 'resolved'),
(122, 83, '2023-12-08 12:48:42.771146', 'Direct Connect (Network, EU-DE, dc) moved to new incident', 'SYSTEM'),
(123, 83, '2023-12-08 13:35:36.467529', 'Minor issues due to the introduction of new Status Dashboard.', 'resolved'),
(124, 84, '2023-12-08 15:49:55.673447', 'Issues due to the introduction of new Status Dashboard.', 'resolved'),
(125, 85, '2023-12-08 15:50:20.670035', 'Minor issues due to the introduction of new Status Dashboard.', 'resolved'),
(126, 86, '2023-12-09 08:38:12.776951', '<Component 212: ModelArts> moved to <Incident 87: Incident (ModelArts)>', 'SYSTEM'),
(127, 87, '2023-12-09 08:38:12.776951', '<Component 212: ModelArts> moved from <Incident 86: Incident>', 'SYSTEM'),
(128, 87, '2023-12-09 08:38:50.264859', 'Minor issues due to the introduction of new Status Dashboard.', 'resolved'),
(129, 88, '2023-12-09 22:42:23.480153', 'Minor issues due to the introduction of new Status Dashboard.', 'resolved'),
(130, 86, '2023-12-09 22:43:02.195725', '<Component 207: Image Management Service> moved to <Incident 89: Incident (Image Management Service)>', 'SYSTEM'),
(131, 89, '2023-12-09 22:43:02.195725', '<Component 207: Image Management Service> moved from <Incident 86: Incident>', 'SYSTEM'),
(132, 89, '2023-12-09 22:43:10.657946', 'Minor issues due to the introduction of new Status Dashboard.', 'resolved'),
(133, 86, '2023-12-10 16:42:07.156558', 'ModelArts (Big Data and Data Analysis, EU-DE, ma) moved to new incident', 'SYSTEM'),
(134, 90, '2023-12-10 20:45:58.851463', 'Minor issues due to the introduction of new Status Dashboard.', 'resolved'),
(135, 86, '2023-12-10 20:46:12.116503', 'Minor issues due to the introduction of new Status Dashboard.', 'resolved'),
(136, 91, '2023-12-11 07:19:59.948833', 'Minor issues due to the introduction of new Status Dashboard.', 'resolved'),
(137, 92, '2023-12-11 07:20:11.894962', 'Minor issues due to the introduction of new Status Dashboard.', 'resolved'),
(138, 93, '2023-12-11 13:53:46.980052', 'Minor issues due to the introduction of new Status Dashboard.', 'resolved'),
(139, 94, '2023-12-11 21:43:39.143286', 'Minor issues due to the introduction of new Status Dashboard.', 'resolved'),
(140, 94, '2023-12-11 21:43:43.929803', 'Minor issues due to the introduction of new Status Dashboard.', 'resolved'),
(141, 95, '2023-12-12 11:26:19.635462', 'Resolved. Minor issues due to the setup of metrics DB.', 'resolved'),
(142, 96, '2023-12-12 11:59:27.065184', 'Maintenance completed.', 'completed'),
(143, 98, '2023-12-12 15:53:31.259766', 'Closing this Maintenance due to parallel one for EVS (Duplicate).', 'completed'),
(144, 99, '2023-12-12 17:02:03.685261', 'Minor issues due to the introduction of new Status Dashboard.\r\n', 'resolved'),
(145, 100, '2023-12-12 21:53:05.746181', 'Minor issues due to the introduction of new Status Dashboard.', 'resolved'),
(146, 101, '2023-12-13 08:44:36.195708', 'Resolved. Minor issues due to the setup of metrics DB.', 'resolved'),
(147, 102, '2023-12-13 18:56:52.954848', 'Minor issues due to the introduction of new Status Dashboard.', 'resolved'),
(148, 103, '2023-12-14 10:25:40.850729', 'Maintenance started.', 'in progress'),
(149, 104, '2023-12-14 11:35:58.587389', 'Minor issues due to the introduction of new Status Dashboard.', 'resolved'),
(120, 82, '2023-12-07 20:16:33.589694', 'As a preparation for IPv6, the physical hosts need to be updated. During the timeframe, you may recognize a higher latency for a very short time.', 'in progress'),
(150, 105, '2023-12-14 22:47:04.535993', 'Minor issues due to the introduction of new Status Dashboard.', 'resolved'),
(151, 106, '2023-12-15 08:27:37.20074', 'Minor issue resolved.', 'resolved'),
(152, 107, '2023-12-16 09:58:49.296925', 'Minor issues due to the introduction of new Status Dashboard.', 'resolved'),
(153, 108, '2023-12-16 22:17:44.607192', 'Minor issues due to the introduction of new Status Dashboard.', 'resolved'),
(154, 109, '2023-12-17 19:55:03.491125', 'Minor issues due to the introduction of new Status Dashboard.', 'resolved'),
(155, 112, '2024-01-18 17:27:59.852748', 'Incident resolved', 'resolved'),
(156, 113, '2024-01-21 10:27:37.331468', 'Service was accessible. Response time just a bit increased for some seconds.', 'resolved'),
(157, 114, '2024-01-24 09:10:43.533737', 'unreasonable request.', 'resolved'),
(158, 114, '2024-01-24 09:10:53.039121', 'unreasonable request.', 'resolved'),
(159, 116, '2024-01-26 08:57:06.010073', 'Unreasonable request', 'resolved'),
(160, 116, '2024-01-26 08:57:08.476845', 'Unreasonable request', 'resolved'),
(161, 116, '2024-01-26 08:57:10.508123', 'Unreasonable request', 'resolved'),
(162, 116, '2024-01-26 08:57:12.918346', 'Unreasonable request', 'resolved'),
(163, 121, '2024-01-30 15:32:04.242641', 'issue resolved.', 'resolved'),
(164, 129, '2024-02-05 07:45:16.799301', 'unreasonable request', 'resolved'),
(165, 129, '2024-02-05 07:45:35.214636', 'unreasonable request', 'resolved'),
(166, 129, '2024-02-05 07:45:43.709174', 'unreasonable request', 'resolved'),
(167, 130, '2024-02-05 16:40:49.348999', 'Unreasonable request', 'resolved'),
(168, 130, '2024-02-05 16:41:00.565392', 'Unreasonable request', 'resolved'),
(169, 130, '2024-02-05 16:41:17.319221', 'Unreasonable request', 'resolved'),
(170, 131, '2024-02-09 07:19:50.164831', 'Unreasonable request', 'resolved'),
(171, 132, '2024-03-01 08:15:39.356428', 'Incident resolved.', 'resolved'),
(172, 135, '2024-03-11 09:20:00.630574', 'done done done', 'resolved'),
(173, 136, '2024-03-13 16:49:36.230502', 'Impact changed: from minor to major', 'SYSTEM'),
(174, 136, '2024-03-13 21:07:33.46179', 'done done done', 'resolved'),
(175, 137, '2024-03-18 12:44:29.810015', 'API slow for some seconds. Resolved.', 'resolved'),
(181, 143, '2024-03-25 20:53:05.167923', 'This incident has been resolved.\r\nThe services were checked and the metrics shown no errors. The healtbars stayed green the whole time.', 'resolved'),
(182, 144, '2024-03-26 08:37:17.398013', 'The services were checked and the metrics show no errors. The healthbars stayed green the whole time.', 'resolved'),
(183, 144, '2024-03-26 08:38:05.236434', 'The services were checked and the metrics show no errors. The healthbars stayed green the whole time.', 'resolved'),
(176, 138, '2024-03-19 16:36:01.134225', 'API went back after a short interrpution', 'resolved'),
(177, 139, '2024-03-19 19:51:23.834463', 'API success rate went down for two minutes.', 'resolved'),
(178, 141, '2024-03-22 07:23:11.474414', 'API response was slow for a minute between 11:00 PM an 11:15 PM.', 'resolved'),
(179, 141, '2024-03-22 07:23:15.587911', 'API response was slow for a minute between 11:00 PM an 11:15 PM.', 'resolved'),
(180, 142, '2024-03-24 07:22:04.488374', 'This incident has been resolved.', 'resolved'),
(184, 145, '2024-03-26 12:59:05.115549', 'This incident has been resolved. The service was checked and the metrics shown no errors. We apologise for any inconvenience that this may have caused.', 'resolved'),
(185, 147, '2024-04-04 07:47:39.447398', 'Incident Resolved', 'resolved'),
(186, 148, '2024-04-04 16:55:49.583748', 'Incident Resolved', 'resolved'),
(187, 148, '2024-04-04 16:56:06.793994', 'Incident Resolved', 'resolved'),
(188, 149, '2024-04-05 05:15:22.094454', 'Incident Resolved', 'resolved'),
(189, 149, '2024-04-05 05:15:52.733056', 'Incident Resolved', 'resolved'),
(190, 150, '2024-04-05 07:44:03.149201', 'Incident Resolved', 'resolved'),
(191, 150, '2024-04-05 07:44:14.275002', 'Incident Resolved', 'resolved'),
(192, 151, '2024-04-05 12:31:26.324532', 'Incident Resolved', 'resolved'),
(193, 146, '2024-04-05 12:35:30.600705', 'Incident Resolved', 'resolved'),
(194, 152, '2024-04-09 06:28:48.463307', 'Resolved on 2024-04-05 around lunch.', 'resolved'),
(195, 153, '2024-04-11 09:06:13.698194', 'Minor hick-up of <1 min. Issue resolved.', 'resolved'),
(196, 155, '2024-04-11 12:37:18.00014', 'resolved\r\n', 'resolved'),
(197, 156, '2024-04-12 10:47:51.379043', 'CES service works for our clients even if the health status on our status dashboard shows something different. We are working on it to solve the problem.', 'analyzing'),
(198, 157, '2024-04-12 14:29:18.865951', 'Due to a planned maintenance at central devices, mainly all services might be interrupted for some seconds during the maintenance window.\r\nTime window UTC:\r\n15.04.2024 between 08:00 PM and 16.04.2024 00:00 AM\r\n\r\nTime window CEST:\r\n15.04.2024 between 10:00 PM and 16.04.2024 02:00 AM\r\n', 'description'),
(199, 159, '2024-04-16 08:07:50.166487', 'Only in case of a problem with the upgrade slower response service interruptions (5xx return codes) and eventually in a most serious case service downtime can occur minor increase of latency can occur', 'description'),
(200, 158, '2024-04-16 14:51:49.3843', 'ruleset adjusted.', 'resolved'),
(201, 156, '2024-04-16 14:53:04.655383', 'Ruleset adjusted. Topic solved and under control.', 'resolved'),
(202, 160, '2024-04-17 12:53:20.639104', 'During the maintenance, life cycle management activities for CCE resources, including modifications, creations, or deletions of clusters or worker nodes may experience disruptions within the maintenance window for brief intervals.', 'description'),
(203, 161, '2024-04-17 13:59:43.896059', 'Due to a planned maintenance at central devices, mainly all services might be interrupted for some seconds during the maintenance window. \r\nTime window UTC: 18.04.2024 between 08:00 PM and 19.04.2024 00:00 AM \r\nTime window CEST: 18.04.2024 between 10:00 PM and 19.04.2024 02:00 AM\r\n', 'description'),
(204, 162, '2024-04-19 04:17:14.681104', 'solved solved solved solved', 'resolved'),
(205, 162, '2024-04-19 04:17:21.96301', 'solved solved solved solved', 'resolved'),
(206, 162, '2024-04-19 04:18:37.606064', 'solved solved solved solved', 'resolved'),
(207, 163, '2024-04-25 16:21:04.331743', 'Issue resolved.\r\nWe had a temporary problem to create/stop/reboot ECS instances.\r\nRunning instances were not affected at all', 'resolved'),
(208, 164, '2024-04-29 06:02:02.469274', 'Increased response time in EU-NL for some minutes.', 'resolved'),
(209, 165, '2024-04-29 13:45:52.063015', 'Please be informed that there will be a scheduled maintenance on KMS service. No downtime is expected as part of a normal operation.\r\nIn case of issues afterwards, please contact the Open Telekom Cloud Service Desk.', 'description'),
(212, 167, '2024-05-06 11:57:08.053836', 'During the maintenance work you can face with some short service outages.\r\n\r\n', 'description'),
(213, 168, '2024-05-06 12:05:54.176772', 'False alarm,\r\nDMS is implementing new monitoring metric and there might occur some short false positives', 'analyzing'),
(214, 168, '2024-05-06 13:47:16.099488', 'New metric. Adjustment was needed.', 'resolved'),
(215, 169, '2024-05-07 10:51:28.408499', 'Incident Resolved', 'resolved'),
(216, 170, '2024-05-13 12:45:42.53724', 'Please be informed that there will be a scheduled maintenance on CBR service. No downtime is expected as part of a normal operation.', 'description'),
(217, 171, '2024-05-14 15:03:38.840858', 'Incident Resolved', 'resolved'),
(218, 172, '2024-05-15 08:36:39.547028', 'Please be informed that there will be scheduled maintenances on Database Services. \r\nDuring the maintenance work you may face with some short service outage period between 8am and 18h CET.\r\nThank you for your understanding.', 'description'),
(219, 173, '2024-05-15 10:13:17.576979', 'Please be informed that there will be scheduled maintenances on CBR, CSBS and VBS services. No service interruption is expected.\r\n', 'description'),
(220, 174, '2024-05-17 10:37:25.052063', 'This maintenance is needed to close a vulnerability.\r\nThere is a minor risk of impact for the customers. It could happen that your persistent connection will get interrupted, and you need to re-connect it.', 'description'),
(221, 175, '2024-05-17 10:38:25.676022', 'This maintenance is needed to close a vulnerability.\r\nThere is a minor risk of impact for the customers. It could happen that your persistent connection will get interrupted, and you need to re-connect it.', 'description'),
(222, 176, '2024-05-17 10:39:20.566792', 'This maintenance is needed to close a vulnerability.\r\nThere is a minor risk of impact for the customers. It could happen that your persistent connection will get interrupted, and you need to re-connect it.\r\n', 'description'),
(223, 177, '2024-05-17 10:40:04.823251', 'This maintenance is needed to close a vulnerability.\r\nThere is a minor risk of impact for the customers. It could happen that your persistent connection will get interrupted, and you need to re-connect it.', 'description'),
(224, 178, '2024-05-18 07:42:17.206664', 'There was a Major incident on sd2 regarding the RTS service in EU-DE region. The service had no problem from Grafana side.', 'resolved'),
(225, 178, '2024-05-18 07:42:25.282402', 'The service had no problem from Grafana side.', 'resolved'),
(226, 179, '2024-05-22 13:40:16.386431', 'This maintenance is needed to close a vulnerability. There is a minor risk of impact for the customers. It could happen that your persistent connection will get interrupted, and you need to re-connect it.', 'description'),
(227, 180, '2024-05-22 13:42:19.01611', 'This maintenance is needed to close a vulnerability. There is a minor risk of impact for the customers. It could happen that your persistent connection will get interrupted, and you need to re-connect it.', 'description'),
(228, 179, '2024-05-22 13:42:19.024213', '<Component 270: Elastic Load Balancing> moved to <Incident 180: Maintenance on Elastic Load Balancer>, Incident closed by system', 'SYSTEM'),
(229, 180, '2024-05-22 13:42:19.024213', '<Component 270: Elastic Load Balancing> moved from <Incident 179: Maintenance on Elastic Load Balancer>', 'SYSTEM'),
(230, 181, '2024-05-26 06:32:29.218205', 'resolved resolved resolved', 'resolved'),
(231, 181, '2024-05-26 06:32:44.477128', 'resolved resolved resolved', 'resolved'),
(232, 181, '2024-05-26 06:33:27.176916', 'resolved resolved resolved', 'resolved'),
(233, 182, '2024-05-26 14:22:51.213833', 'done done done', 'resolved'),
(234, 183, '2024-05-27 06:11:51.637169', 'done done done', 'resolved'),
(235, 183, '2024-05-27 06:12:40.381252', 'done done done', 'resolved'),
(267, 206, '2024-07-15 13:55:13.341104', 'Topic fixed', 'resolved'),
(236, 184, '2024-05-28 09:27:18.121155', 'Running VMs are not affected, but creation/modification of VMs might not work for some pods in mentioned timeframe. A pod is a organisational unit of hardware in OTC.', 'description'),
(237, 185, '2024-05-28 09:45:53.158278', 'Running VMs are not affected, but creation/modification of ECSs might not work for some pods in mentioned timeframe. A pod is a organisational unit of hardware in OTC.', 'description'),
(238, 184, '2024-05-28 09:47:09.994249', 'Closed this maintenance and created 185. The assignment to services was wrong and now corrected.', 'completed'),
(239, 186, '2024-05-28 13:11:45.101248', 'During the implementation, several services might be impacted for only a short period of time (seconds or maximum a few minutes).', 'description'),
(240, 187, '2024-05-31 06:26:28.201667', 'Minor INC closed, Ops worked it down.', 'resolved'),
(241, 188, '2024-06-04 07:12:19.722137', 'No active issue', 'resolved'),
(242, 189, '2024-06-05 14:15:56.952746', 'issue resolved.', 'resolved'),
(243, 190, '2024-06-06 15:13:32.155964', 'no issue. Service was stable.', 'resolved'),
(244, 191, '2024-06-10 11:49:38.371939', 'only a 1 min peak. Issue resolved.', 'resolved'),
(245, 192, '2024-06-10 12:01:26.792906', 'During our maintenance work you may face with some-seconds service interruption in the defined time window (09:00 AM to 12:00 PM CET) for MySQL, Postgre SQL and MSSQL.', 'description'),
(246, 186, '2024-06-10 12:01:26.797748', '<Component 224: Relational Database Service> moved to <Incident 192: RDS Database Service Maintenance>', 'SYSTEM'),
(247, 192, '2024-06-10 12:01:26.797748', '<Component 224: Relational Database Service> moved from <Incident 186: Openstack Upgrade in Region: EU-DE>', 'SYSTEM'),
(248, 193, '2024-06-14 18:01:57.499921', 'Please be informed that there will be a maintenance on Virtual Private Cloud service. No downtime is expected.', 'description'),
(249, 194, '2024-06-14 18:03:47.738817', 'Please be informed that there will be a maintenance work on Elastic Load Balancer service. No downtime is expected.', 'description'),
(250, 195, '2024-06-18 13:33:25.473261', 'Please be informed that there will be a scheduled Maintenance on the VPC and ELB services. No downtimes are expected.', 'description'),
(251, 196, '2024-06-19 10:59:17.623847', 'Please be informed that we are performing a maintenance on CCE services, during this time the upgrade function is temporarily unavailable.', 'description'),
(252, 197, '2024-06-19 15:09:56.863873', 'Please be informed that there will be a scheduled Maintenance on the VPC and ELB services. No downtimes are expected.', 'description'),
(253, 195, '2024-06-19 15:09:56.873049', 'Elastic Load Balancing (Network, EU-NL, elb) moved to <a href="/incidents/197">Maintenance on VPC and ELB Services</a>', 'SYSTEM'),
(254, 197, '2024-06-19 15:09:56.873049', 'Elastic Load Balancing (Network, EU-NL, elb) moved from <a href="/incidents/195">Maintenance on VPC and ELB Services</a>', 'SYSTEM'),
(255, 198, '2024-06-21 07:37:36.214979', 'Please be informed that there will be a scheduled Maintenance on the VPC and ELB services. No downtimes are expected.', 'description'),
(256, 199, '2024-06-24 12:06:37.147829', 'Incident Resolved.', 'resolved'),
(257, 200, '2024-06-28 07:09:50.278477', 'During the implementation, several services might be impacted for a very short period of time (seconds or maximum a few minutes).', 'description'),
(258, 186, '2024-06-28 07:09:50.286643', 'Elastic Cloud Server (Compute, EU-DE, ecs) moved to <a href="/incidents/200">OpenStack Upgrade in regions EU-DE/EU-NL</a>, Elastic Volume Service (Storage, EU-DE, evs) moved to <a href="/incidents/200">OpenStack Upgrade in regions EU-DE/EU-NL</a>, Object Storage Service (Storage, EU-DE, obs) moved to <a href="/incidents/200">OpenStack Upgrade in regions EU-DE/EU-NL</a>, Incident closed by system', 'SYSTEM'),
(259, 200, '2024-06-28 07:09:50.286643', 'Elastic Cloud Server (Compute, EU-DE, ecs) moved from <a href="/incidents/186">Openstack Upgrade in Region: EU-DE</a>, Elastic Volume Service (Storage, EU-DE, evs) moved from <a href="/incidents/186">Openstack Upgrade in Region: EU-DE</a>, Object Storage Service (Storage, EU-DE, obs) moved from <a href="/incidents/186">Openstack Upgrade in Region: EU-DE</a>', 'SYSTEM'),
(260, 201, '2024-06-28 13:38:26.554117', 'Small increased response times due to a backend change. No customer impact.', 'resolved'),
(261, 202, '2024-06-28 13:37:53.937561', 'Small peak, but no outage', 'resolved'),
(262, 203, '2024-07-10 07:20:36.95944', 'minor incrase of resonse time within the mentioned time frame.', 'resolved'),
(263, 204, '2024-07-10 07:20:36.95944', 'The maintenance may result in a short interruption for a few seconds. We rather expect a short slowness, but no downtime.', 'description'),
(264, 200, '2024-07-10 07:20:36.95944', 'Elastic Cloud Server (Compute, EU-DE, ecs) moved to <a href="/incidents/204">Scheduled Maintenance: potential slowness of some services for some seconds</a>, Elastic Volume Service (Storage, EU-DE, evs) moved to <a href="/incidents/204">Scheduled Maintenance: potential slowness of some services for some seconds</a>', 'SYSTEM'),
(265, 204, '2024-07-10 07:20:36.95944', 'Elastic Cloud Server (Compute, EU-DE, ecs) moved from <a href="/incidents/200">OpenStack Upgrade in regions EU-DE/EU-NL</a>, Elastic Volume Service (Storage, EU-DE, evs) moved from <a href="/incidents/200">OpenStack Upgrade in regions EU-DE/EU-NL</a>', 'SYSTEM'),
(266, 205, '2024-07-10 07:20:36.95944', 'Service is OK. Message is not relevant.', 'resolved'),
(268, 207, '2024-07-15 13:55:43.257406', 'issue resolved now', 'resolved'),
(271, 209, '2024-07-15 13:55:43.257406', 'No impact for end customers.', 'resolved'),
(272, 210, '2024-07-15 13:55:43.257406', 'Dashboard notification issue. Servcie is running fine.', 'resolved'),
(274, 212, '2024-07-15 13:55:43.257406', 'During the implementation, several services might be impacted for a very short period of time (seconds or maximum a few minutes). In case of issues afterwards, please contact the Open Telekom Cloud Service Desk.', 'description'),
(275, 212, '2024-07-15 13:55:43.257406', 'maintenance moved to 25.07.2024.', 'completed'),
(276, 213, '2024-07-15 13:55:43.257406', 'During the implementation, several services might be impacted for a very short period of time (seconds or maximum a few minutes). In case of issues afterwards, please contact the Open Telekom Cloud Service Desk.', 'description'),
(280, 216, '2024-07-15 13:55:43.257406', 'Elastic Volume Service (Storage, EU-NL, evs) added to Incident', 'SYSTEM'),
(282, 216, '2024-07-15 13:55:43.257406', 'Cloud Trace Service (Management & Deployment, EU-NL, cts) moved from <a href="/incidents/215">Incident</a>', 'SYSTEM'),
(283, 215, '2024-07-15 13:55:43.257406', 'Cloud Trace Service (Management & Deployment, EU-NL, cts) moved to <a href="/incidents/216">Incident</a>', 'SYSTEM'),
(284, 215, '2024-07-15 13:55:43.257406', 'Log Tank Service (Management & Deployment, EU-NL, lts) added to Incident', 'SYSTEM'),
(285, 216, '2024-07-15 13:55:43.257406', 'We found no issues regarding the service', 'resolved'),
(286, 216, '2024-07-15 13:55:43.257406', 'We found no issues regarding the service', 'resolved'),
(287, 215, '2024-07-15 13:55:43.257406', 'We found no issues regarding the service', 'resolved'),
(288, 217, '2024-07-15 13:55:43.257406', 'Service was fully available.', 'resolved'),
(289, 218, '2024-07-15 13:55:43.257406', 'Service working fine. Issue with monitoring engine.', 'resolved'),
(290, 219, '2024-07-15 13:55:43.257406', 'During our Maintenance work you may face some very short service outages', 'description'),
(291, 220, '2024-07-15 13:55:43.257406', 'During the implementation, several services might be impacted for a very short period of time (seconds or maximum a few minutes). In case of issues afterwards, please contact the Open Telekom Cloud Service Desk.\r\n\r\nIn case of issues afterwards, please contact the Open Telekom Cloud Service Desk.', 'description'),
(269, 208, '2024-07-15 13:55:13.341104', 'We found no issues regarding the service.', 'resolved'),
(270, 208, '2024-07-15 13:55:13.341104', 'We found no issues regarding the service.', 'resolved'),
(273, 211, '2024-07-15 13:55:13.341104', 'Resolved. Issue during the integration of a new service. No technical impact to EVS.', 'resolved'),
(277, 214, '2024-07-15 13:55:13.341104', 'Affected Services: OBS, ModelArts, CCE, ECS, SFS 3.0, AOM, CSS, CTS, DCS, DLI, DMS, DWS, FunctionGraph, GES, LTS, DeH, DC, EVS, IMS, ROS, DBSS, WAF\r\nCustomer Impact: Creating new resources might fail, as the IAM write APIs are not available for approx. 10 minutes. Retrying the operation after 10 minutes will solve the issue.', 'description'),
(278, 200, '2024-07-15 13:55:13.341104', 'Elastic Cloud Server (Compute, EU-NL, ecs) moved to <a href='/incidents/214'>IAM Maitenanace</a>, Elastic Volume Service (Storage, EU-NL, evs) moved to <a href='/incidents/214'>IAM Maitenanace</a>, Object Storage Service (Storage, EU-NL, obs) moved to <a href='/incidents/214'>IAM Maitenanace</a>, Object Storage Service (Storage, EU-DE, obs) moved to <a href='/incidents/214'>IAM Maitenanace</a>, Incident closed by system', 'SYSTEM'),
(279, 214, '2024-07-15 13:55:13.341104', 'Elastic Cloud Server (Compute, EU-NL, ecs) moved from <a href='/incidents/200'>OpenStack Upgrade in regions EU-DE/EU-NL</a>, Elastic Volume Service (Storage, EU-NL, evs) moved from <a href='/incidents/200'>OpenStack Upgrade in regions EU-DE/EU-NL</a>, Object Storage Service (Storage, EU-NL, obs) moved from <a href='/incidents/200'>OpenStack Upgrade in regions EU-DE/EU-NL</a>, Object Storage Service (Storage, EU-DE, obs) moved from <a href='/incidents/200'>OpenStack Upgrade in regions EU-DE/EU-NL</a>', 'SYSTEM'),
(281, 215, '2024-07-25 19:58:51.72496', 'Dedicated Host (Compute, EU-NL, deh) added to Incident', 'SYSTEM'),
(295, 224, '2024-07-25 19:58:51.72496', 'Checked. No real issue. INM will be closed', 'resolved'),
(292, 221, '2024-07-15 13:55:43.257406', 'During the Maintenance work you can expect short service outages', 'description'),
(293, 222, '2024-07-15 13:55:43.257406', 'Scheduled DB Maintenance for ModelArts. \r\nShort Interruptions can be expected in the mentioned timeframe.', 'description'),
(294, 223, '2024-07-15 13:55:43.257406', 'issue in data point measuring. Service was stable.', 'resolved'),
(296, 225, '2024-07-15 13:55:43.257406', 'Checked. No real Issue. INM will be closed.', 'resolved'),
(297, 226, '2024-07-15 13:55:43.257406', 'service working fine. Issue in status processor.', 'resolved'),
(298, 227, '2024-08-06 21:40:49.419671', 'Please be informed that there will be a maintenance work on AZ3 of EU-DE region on RDS service.\r\nDuring this time you may face some-seconds of service outage.', 'description'),
(299, 228, '2024-08-06 21:40:49.419671', 'Re-routing of OBS traffic due to architectural backend changes.\r\nImpact: OBS connection may get interrupted for a couple of seconds.', 'description'),
(300, 229, '2024-08-06 21:41:19.074572', 'No customer impact. Issue resolved.', 'resolved'),
(301, 230, '2024-08-06 21:41:19.074572', 'No customer impact. Issue resolved.', 'resolved'),
(302, 231, '2024-08-06 21:40:49.419671', 'Issue solved', 'resolved'),
(303, 232, '2024-08-13 06:08:52.383117', 'Issue resolved.', 'resolved'),
(304, 233, '2024-08-13 06:08:52.383117', 'Re-routing of OBS traffic due to architectural backend changes. Impact: OBS connection may get interrupted for a couple of seconds.', 'description'),
(305, 234, '2024-08-06 21:40:49.419671', 'Please be informed that there will be a scheduled maintenance on CBR service. No downtime is expected as part of a normal operation.', 'description'),
(306, 235, '2024-08-06 21:40:49.419671', 'Issues identified and solved.', 'resolved'),
(307, 236, '2024-08-13 06:08:52.383117', 'Please be informed that there will be a scheduled maintenance on CBR service. No downtime is expected as part of a normal operation.', 'description'),
(308, 234, '2024-08-13 06:08:52.383117', 'Cloud Backup and Recovery (Storage, EU-NL, cbr) moved to <a href='/incidents/236'>Maintenance on CBR Services</a>, Incident closed by system', 'SYSTEM'),
(309, 236, '2024-08-13 06:08:52.383117', 'Cloud Backup and Recovery (Storage, EU-NL, cbr) moved from <a href='/incidents/234'>Maintenance on CBR Services</a>', 'SYSTEM'),
(310, 237, '2024-08-06 21:40:49.419671', 'Issue solved.', 'resolved');


--
-- Name: component_attribute_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.component_attribute_id_seq', 864, true);


--
-- Name: component_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.component_id_seq', 288, true);


--
-- Name: incident_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.incident_id_seq', 237, true);


--
-- Name: incident_status_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.incident_status_id_seq', 310, true);


--
-- Name: alembic_version alembic_version_pkc; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.alembic_version
    ADD CONSTRAINT alembic_version_pkc PRIMARY KEY (version_num);


--
-- Name: component_attribute component_attribute_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.component_attribute
    ADD CONSTRAINT component_attribute_pkey PRIMARY KEY (id);


--
-- Name: component component_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.component
    ADD CONSTRAINT component_pkey PRIMARY KEY (id);


--
-- Name: incident incident_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.incident
    ADD CONSTRAINT incident_pkey PRIMARY KEY (id);


--
-- Name: incident_status incident_status_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.incident_status
    ADD CONSTRAINT incident_status_pkey PRIMARY KEY (id);


--
-- Name: inc_comp_rel; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX inc_comp_rel ON public.incident_component_relation USING btree (incident_id, component_id);


--
-- Name: ix_component_attribute_component_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_component_attribute_component_id ON public.component_attribute USING btree (component_id);


--
-- Name: ix_component_attribute_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_component_attribute_id ON public.component_attribute USING btree (id);


--
-- Name: ix_component_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_component_id ON public.component USING btree (id);


--
-- Name: ix_incident_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_incident_id ON public.incident USING btree (id);


--
-- Name: ix_incident_status_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_incident_status_id ON public.incident_status USING btree (id);


--
-- Name: ix_incident_status_incident_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_incident_status_incident_id ON public.incident_status USING btree (incident_id);


--
-- Name: component_attribute component_attribute_component_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.component_attribute
    ADD CONSTRAINT component_attribute_component_id_fkey FOREIGN KEY (component_id) REFERENCES public.component(id);


--
-- Name: incident_component_relation incident_component_relation_component_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.incident_component_relation
    ADD CONSTRAINT incident_component_relation_component_id_fkey FOREIGN KEY (component_id) REFERENCES public.component(id);


--
-- PostgreSQL database dump complete
--

