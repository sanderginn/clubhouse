-- Create sections table
CREATE TABLE sections (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name VARCHAR(255) NOT NULL,
  type VARCHAR(50) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT now()
);

-- Create indexes for sections table
CREATE INDEX idx_sections_type ON sections(type);

-- Insert default sections
INSERT INTO sections (name, type) VALUES
  ('Music', 'music'),
  ('Photos', 'photo'),
  ('Events', 'event'),
  ('Recipes', 'recipe'),
  ('Books', 'book'),
  ('Movies', 'movie'),
  ('General', 'general');
