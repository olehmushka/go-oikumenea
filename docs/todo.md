# Raw ideas to discuss before inserting to milestones

## 1. Add language & language group

Language Group's Fields:
1. Code (Glottolog)
2. Name
3. Parent Language Group ID (optional)

Writing Sysytems
1. Code ISO (ISO 15924)
2. Name
3. Script type: Logographic, Syllabaries, Alphabets, some other?

Language's Fields:
1. Code ISO (ISO 639-3)
2. Code Glottolog (Glottolog)
3. Name
4. LanguageGroup
5. Living (boolean)
6. Writing systems (some are primary, some are not)

The link between persons and languages (using level of use: A1, A2, B1, B2, C1, C2; native: boolean)
And maybe apply it to existing objects where languages are applicable.

## 2. Education information

I want to add some education information:
1. The kindergartens, the schools, the colleges, the institutes, the universities, the academies, etc
2. The buildings of the entities from point 1 (so the location is required)
3. The campuses, the departments, the institutes of universities, etc (all educational structure should be reflected here)
4. I want to make the best binding of persons to the eduction domain
    1. when they were students
    2. who were their professors/tututor/curator
    3. what the university groups they belonged
    4. what the departments they studied
5. the locations of their dormitories (the appartments/houses or universities/colleges for living of their students)
6. the university/schools/etc have possitions and persons may occupy these positions (like a military)

Location Fields (just proposal, they can be completly different. the main emphesy on detalization and better information for further analytics & building relations and graphhs):
1. wgs84 coordinate (coordinates         GEOGRAPHY(POINT, 4326))
2. mgrs coordinates (derivates from wgs84 coordinate)
3. h3 indexes
4. raw_address         TEXT,
5. country_code        TEXT NOT NULL                  -- ISO 3166-1 alpha-2
6. admin_area_1        TEXT,                          -- state / oblast
7. admin_area_2        TEXT,                          -- county / raion
8. locality            TEXT,                          -- city / town
9. street              TEXT,
10. house_number        TEXT,
11. postal_code         TEXT,

## 3. Companies

I want to hold informations about companies (private, public, state-owned, etc)
i am not sure all things i want to have here but it should be all possible information (like ukrainian youcontrol does)
it should have relations with locations
it should contain country registration of the company,
address of registration
type of the company,
domain of the company,
possitions of the company,
etc
actually i want to discuss it with you about fields and the purposes (i want you to ask me all things to make it correct)

## 4. Religion

There are a lot of to discuss. Let's postone it.
