-- Факты
CREATE TABLE facts (
    id UUID PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    content TEXT NOT NULL,
    source_url TEXT,
    confidence DECIMAL(3,2) DEFAULT 1.0,
    extracted_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX idx_facts_user ON facts(user_id);
CREATE INDEX idx_facts_content_search ON facts USING gin(to_tsvector('russian', content));

-- Сущности
CREATE TABLE entities (
    id UUID PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    first_seen_at TIMESTAMP DEFAULT NOW(),
    last_seen_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, name)
);

-- Атрибуты сущностей (key-value)
CREATE TABLE entity_attributes (
    entity_id UUID REFERENCES entities(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    PRIMARY KEY (entity_id, key)
);

-- Сессии исследований
CREATE TABLE research_sessions (
    id UUID PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    question TEXT NOT NULL,
    strategy TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Связи many-to-many
CREATE TABLE session_facts (
    session_id UUID REFERENCES research_sessions(id),
    fact_id UUID REFERENCES facts(id),
    PRIMARY KEY (session_id, fact_id)
);

CREATE TABLE session_entities (
    session_id UUID REFERENCES research_sessions(id),
    entity_id UUID REFERENCES entities(id),
    PRIMARY KEY (session_id, entity_id)
);
