-- Create section_subscriptions table (opt-out tracking)
CREATE TABLE section_subscriptions (
  user_id UUID NOT NULL REFERENCES users(id),
  section_id UUID NOT NULL REFERENCES sections(id),
  opted_out_at TIMESTAMP NOT NULL DEFAULT now(),

  PRIMARY KEY (user_id, section_id)
);

-- Create indexes for section_subscriptions table
CREATE INDEX idx_section_subs_user_id ON section_subscriptions(user_id);
CREATE INDEX idx_section_subs_section_id ON section_subscriptions(section_id);
