-- 0000 schema bootstrap (M0 walking skeleton).
--
-- Creates the shared `oikumenea` SQL objects every module depends on, BEFORE any module
-- table (ordering invariant; docs/modules/platform.md). Expand-only (L-UpgradeSafe / D-Migrations).
-- Owns: schema + extensions, uuid_v7(), new_rid() (D-ResourceIdentifiers), set_updated_at(),
-- reject_mutation(), the single-row schema_version marker, and the seeded ISO-3166-1 alpha-2
-- geo_countries registry (D-Geo).

CREATE SCHEMA IF NOT EXISTS oikumenea;

CREATE EXTENSION IF NOT EXISTS citext;   -- case-insensitive text (e.g. account emails)
CREATE EXTENSION IF NOT EXISTS pgcrypto; -- gen_random_bytes() for uuid_v7()

-- uuid_v7(): time-ordered UUIDv7 (RFC 9562). The crypto component inside every RID; also
-- gives B-tree insert locality. PG16 has no built-in uuidv7(), so we generate it.
CREATE OR REPLACE FUNCTION oikumenea.uuid_v7() RETURNS uuid
  LANGUAGE plpgsql VOLATILE PARALLEL SAFE AS $$
DECLARE
  unix_ts_ms bigint;
  b bytea;
BEGIN
  unix_ts_ms := floor(extract(epoch FROM clock_timestamp()) * 1000)::bigint;
  b := gen_random_bytes(16);
  -- first 48 bits = big-endian unix millis (low 6 bytes of the 8-byte int)
  b := overlay(b PLACING substring(int8send(unix_ts_ms) FROM 3 FOR 6) FROM 1 FOR 6);
  -- byte 7: version nibble = 0x7
  b := set_byte(b, 6, (get_byte(b, 6) & 15) | 112);
  -- byte 9: variant bits = 0b10
  b := set_byte(b, 8, (get_byte(b, 8) & 63) | 128);
  RETURN encode(b, 'hex')::uuid;
END;
$$;

-- new_rid(): composes the self-describing URN RID used as every PK default
-- (D-ResourceIdentifiers). <environment> comes from the per-session app.environment GUC.
CREATE OR REPLACE FUNCTION oikumenea.new_rid(service text, entity_type text) RETURNS text
  LANGUAGE sql VOLATILE AS $$
    SELECT 'urn:oikumenea:' || service || ':' || current_setting('app.environment')
        || ':' || entity_type || ':' || oikumenea.uuid_v7()::text
$$;

-- set_updated_at(): BEFORE UPDATE trigger keeping updated_at current.
CREATE OR REPLACE FUNCTION oikumenea.set_updated_at() RETURNS trigger
  LANGUAGE plpgsql AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$$;

-- reject_mutation(): BEFORE UPDATE OR DELETE guard for append-only tables (audit, events).
CREATE OR REPLACE FUNCTION oikumenea.reject_mutation() RETURNS trigger
  LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'append-only table %.%: % is not permitted',
    TG_TABLE_SCHEMA, TG_TABLE_NAME, TG_OP USING ERRCODE = 'restrict_violation';
END;
$$;

-- schema_version: single-row marker the boot-time check reads (upgrade-safety.md).
CREATE TABLE oikumenea.schema_version (
  singleton  boolean PRIMARY KEY DEFAULT true CHECK (singleton),
  revision   text NOT NULL,
  applied_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO oikumenea.schema_version (singleton, revision) VALUES (true, '0000_schema_bootstrap');

-- geo_countries: seeded ISO-3166-1 alpha-2 registry (D-Geo). Natural code PK (not an RID,
-- per D-ResourceIdentifiers carve-out). Default-locale (English) name; other locales arrive
-- via the i18n store (M2). Instance-admin-extensible (country.manage).
CREATE TABLE oikumenea.geo_countries (
  code       char(2) PRIMARY KEY,
  name       text NOT NULL,
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order integer,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER geo_countries_set_updated_at
  BEFORE UPDATE ON oikumenea.geo_countries
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.geo_countries.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.geo_countries.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.geo_countries.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.geo_countries.sort_order IS 'pii:none';

INSERT INTO oikumenea.geo_countries (code, name, sort_order) VALUES
  ('AD', 'Andorra', 0),
  ('AE', 'United Arab Emirates', 10),
  ('AF', 'Afghanistan', 20),
  ('AG', 'Antigua and Barbuda', 30),
  ('AI', 'Anguilla', 40),
  ('AL', 'Albania', 50),
  ('AM', 'Armenia', 60),
  ('AO', 'Angola', 70),
  ('AQ', 'Antarctica', 80),
  ('AR', 'Argentina', 90),
  ('AS', 'American Samoa', 100),
  ('AT', 'Austria', 110),
  ('AU', 'Australia', 120),
  ('AW', 'Aruba', 130),
  ('AX', 'Åland Islands', 140),
  ('AZ', 'Azerbaijan', 150),
  ('BA', 'Bosnia and Herzegovina', 160),
  ('BB', 'Barbados', 170),
  ('BD', 'Bangladesh', 180),
  ('BE', 'Belgium', 190),
  ('BF', 'Burkina Faso', 200),
  ('BG', 'Bulgaria', 210),
  ('BH', 'Bahrain', 220),
  ('BI', 'Burundi', 230),
  ('BJ', 'Benin', 240),
  ('BL', 'Saint Barthélemy', 250),
  ('BM', 'Bermuda', 260),
  ('BN', 'Brunei Darussalam', 270),
  ('BO', 'Bolivia', 280),
  ('BQ', 'Bonaire, Sint Eustatius and Saba', 290),
  ('BR', 'Brazil', 300),
  ('BS', 'Bahamas', 310),
  ('BT', 'Bhutan', 320),
  ('BV', 'Bouvet Island', 330),
  ('BW', 'Botswana', 340),
  ('BY', 'Belarus', 350),
  ('BZ', 'Belize', 360),
  ('CA', 'Canada', 370),
  ('CC', 'Cocos (Keeling) Islands', 380),
  ('CD', 'Congo, The Democratic Republic of the', 390),
  ('CF', 'Central African Republic', 400),
  ('CG', 'Congo', 410),
  ('CH', 'Switzerland', 420),
  ('CI', 'Côte d''Ivoire', 430),
  ('CK', 'Cook Islands', 440),
  ('CL', 'Chile', 450),
  ('CM', 'Cameroon', 460),
  ('CN', 'China', 470),
  ('CO', 'Colombia', 480),
  ('CR', 'Costa Rica', 490),
  ('CU', 'Cuba', 500),
  ('CV', 'Cabo Verde', 510),
  ('CW', 'Curaçao', 520),
  ('CX', 'Christmas Island', 530),
  ('CY', 'Cyprus', 540),
  ('CZ', 'Czechia', 550),
  ('DE', 'Germany', 560),
  ('DJ', 'Djibouti', 570),
  ('DK', 'Denmark', 580),
  ('DM', 'Dominica', 590),
  ('DO', 'Dominican Republic', 600),
  ('DZ', 'Algeria', 610),
  ('EC', 'Ecuador', 620),
  ('EE', 'Estonia', 630),
  ('EG', 'Egypt', 640),
  ('EH', 'Western Sahara', 650),
  ('ER', 'Eritrea', 660),
  ('ES', 'Spain', 670),
  ('ET', 'Ethiopia', 680),
  ('FI', 'Finland', 690),
  ('FJ', 'Fiji', 700),
  ('FK', 'Falkland Islands (Malvinas)', 710),
  ('FM', 'Micronesia, Federated States of', 720),
  ('FO', 'Faroe Islands', 730),
  ('FR', 'France', 740),
  ('GA', 'Gabon', 750),
  ('GB', 'United Kingdom', 760),
  ('GD', 'Grenada', 770),
  ('GE', 'Georgia', 780),
  ('GF', 'French Guiana', 790),
  ('GG', 'Guernsey', 800),
  ('GH', 'Ghana', 810),
  ('GI', 'Gibraltar', 820),
  ('GL', 'Greenland', 830),
  ('GM', 'Gambia', 840),
  ('GN', 'Guinea', 850),
  ('GP', 'Guadeloupe', 860),
  ('GQ', 'Equatorial Guinea', 870),
  ('GR', 'Greece', 880),
  ('GS', 'South Georgia and the South Sandwich Islands', 890),
  ('GT', 'Guatemala', 900),
  ('GU', 'Guam', 910),
  ('GW', 'Guinea-Bissau', 920),
  ('GY', 'Guyana', 930),
  ('HK', 'Hong Kong', 940),
  ('HM', 'Heard Island and McDonald Islands', 950),
  ('HN', 'Honduras', 960),
  ('HR', 'Croatia', 970),
  ('HT', 'Haiti', 980),
  ('HU', 'Hungary', 990),
  ('ID', 'Indonesia', 1000),
  ('IE', 'Ireland', 1010),
  ('IL', 'Israel', 1020),
  ('IM', 'Isle of Man', 1030),
  ('IN', 'India', 1040),
  ('IO', 'British Indian Ocean Territory', 1050),
  ('IQ', 'Iraq', 1060),
  ('IR', 'Iran', 1070),
  ('IS', 'Iceland', 1080),
  ('IT', 'Italy', 1090),
  ('JE', 'Jersey', 1100),
  ('JM', 'Jamaica', 1110),
  ('JO', 'Jordan', 1120),
  ('JP', 'Japan', 1130),
  ('KE', 'Kenya', 1140),
  ('KG', 'Kyrgyzstan', 1150),
  ('KH', 'Cambodia', 1160),
  ('KI', 'Kiribati', 1170),
  ('KM', 'Comoros', 1180),
  ('KN', 'Saint Kitts and Nevis', 1190),
  ('KP', 'North Korea', 1200),
  ('KR', 'South Korea', 1210),
  ('KW', 'Kuwait', 1220),
  ('KY', 'Cayman Islands', 1230),
  ('KZ', 'Kazakhstan', 1240),
  ('LA', 'Laos', 1250),
  ('LB', 'Lebanon', 1260),
  ('LC', 'Saint Lucia', 1270),
  ('LI', 'Liechtenstein', 1280),
  ('LK', 'Sri Lanka', 1290),
  ('LR', 'Liberia', 1300),
  ('LS', 'Lesotho', 1310),
  ('LT', 'Lithuania', 1320),
  ('LU', 'Luxembourg', 1330),
  ('LV', 'Latvia', 1340),
  ('LY', 'Libya', 1350),
  ('MA', 'Morocco', 1360),
  ('MC', 'Monaco', 1370),
  ('MD', 'Moldova', 1380),
  ('ME', 'Montenegro', 1390),
  ('MF', 'Saint Martin (French part)', 1400),
  ('MG', 'Madagascar', 1410),
  ('MH', 'Marshall Islands', 1420),
  ('MK', 'North Macedonia', 1430),
  ('ML', 'Mali', 1440),
  ('MM', 'Myanmar', 1450),
  ('MN', 'Mongolia', 1460),
  ('MO', 'Macao', 1470),
  ('MP', 'Northern Mariana Islands', 1480),
  ('MQ', 'Martinique', 1490),
  ('MR', 'Mauritania', 1500),
  ('MS', 'Montserrat', 1510),
  ('MT', 'Malta', 1520),
  ('MU', 'Mauritius', 1530),
  ('MV', 'Maldives', 1540),
  ('MW', 'Malawi', 1550),
  ('MX', 'Mexico', 1560),
  ('MY', 'Malaysia', 1570),
  ('MZ', 'Mozambique', 1580),
  ('NA', 'Namibia', 1590),
  ('NC', 'New Caledonia', 1600),
  ('NE', 'Niger', 1610),
  ('NF', 'Norfolk Island', 1620),
  ('NG', 'Nigeria', 1630),
  ('NI', 'Nicaragua', 1640),
  ('NL', 'Netherlands', 1650),
  ('NO', 'Norway', 1660),
  ('NP', 'Nepal', 1670),
  ('NR', 'Nauru', 1680),
  ('NU', 'Niue', 1690),
  ('NZ', 'New Zealand', 1700),
  ('OM', 'Oman', 1710),
  ('PA', 'Panama', 1720),
  ('PE', 'Peru', 1730),
  ('PF', 'French Polynesia', 1740),
  ('PG', 'Papua New Guinea', 1750),
  ('PH', 'Philippines', 1760),
  ('PK', 'Pakistan', 1770),
  ('PL', 'Poland', 1780),
  ('PM', 'Saint Pierre and Miquelon', 1790),
  ('PN', 'Pitcairn', 1800),
  ('PR', 'Puerto Rico', 1810),
  ('PS', 'Palestine, State of', 1820),
  ('PT', 'Portugal', 1830),
  ('PW', 'Palau', 1840),
  ('PY', 'Paraguay', 1850),
  ('QA', 'Qatar', 1860),
  ('RE', 'Réunion', 1870),
  ('RO', 'Romania', 1880),
  ('RS', 'Serbia', 1890),
  ('RU', 'Russian Federation', 1900),
  ('RW', 'Rwanda', 1910),
  ('SA', 'Saudi Arabia', 1920),
  ('SB', 'Solomon Islands', 1930),
  ('SC', 'Seychelles', 1940),
  ('SD', 'Sudan', 1950),
  ('SE', 'Sweden', 1960),
  ('SG', 'Singapore', 1970),
  ('SH', 'Saint Helena, Ascension and Tristan da Cunha', 1980),
  ('SI', 'Slovenia', 1990),
  ('SJ', 'Svalbard and Jan Mayen', 2000),
  ('SK', 'Slovakia', 2010),
  ('SL', 'Sierra Leone', 2020),
  ('SM', 'San Marino', 2030),
  ('SN', 'Senegal', 2040),
  ('SO', 'Somalia', 2050),
  ('SR', 'Suriname', 2060),
  ('SS', 'South Sudan', 2070),
  ('ST', 'Sao Tome and Principe', 2080),
  ('SV', 'El Salvador', 2090),
  ('SX', 'Sint Maarten (Dutch part)', 2100),
  ('SY', 'Syria', 2110),
  ('SZ', 'Eswatini', 2120),
  ('TC', 'Turks and Caicos Islands', 2130),
  ('TD', 'Chad', 2140),
  ('TF', 'French Southern Territories', 2150),
  ('TG', 'Togo', 2160),
  ('TH', 'Thailand', 2170),
  ('TJ', 'Tajikistan', 2180),
  ('TK', 'Tokelau', 2190),
  ('TL', 'Timor-Leste', 2200),
  ('TM', 'Turkmenistan', 2210),
  ('TN', 'Tunisia', 2220),
  ('TO', 'Tonga', 2230),
  ('TR', 'Türkiye', 2240),
  ('TT', 'Trinidad and Tobago', 2250),
  ('TV', 'Tuvalu', 2260),
  ('TW', 'Taiwan', 2270),
  ('TZ', 'Tanzania', 2280),
  ('UA', 'Ukraine', 2290),
  ('UG', 'Uganda', 2300),
  ('UM', 'United States Minor Outlying Islands', 2310),
  ('US', 'United States', 2320),
  ('UY', 'Uruguay', 2330),
  ('UZ', 'Uzbekistan', 2340),
  ('VA', 'Holy See (Vatican City State)', 2350),
  ('VC', 'Saint Vincent and the Grenadines', 2360),
  ('VE', 'Venezuela', 2370),
  ('VG', 'Virgin Islands, British', 2380),
  ('VI', 'Virgin Islands, U.S.', 2390),
  ('VN', 'Vietnam', 2400),
  ('VU', 'Vanuatu', 2410),
  ('WF', 'Wallis and Futuna', 2420),
  ('WS', 'Samoa', 2430),
  ('YE', 'Yemen', 2440),
  ('YT', 'Mayotte', 2450),
  ('ZA', 'South Africa', 2460),
  ('ZM', 'Zambia', 2470),
  ('ZW', 'Zimbabwe', 2480);
