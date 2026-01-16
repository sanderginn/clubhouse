-- Create push_subscriptions table (Web Push API)
CREATE TABLE push_subscriptions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id),
  endpoint TEXT NOT NULL UNIQUE,
  auth_key TEXT NOT NULL,
  p256dh_key TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT now(),
  deleted_at TIMESTAMP
);

-- Create indexes for push_subscriptions table
CREATE INDEX idx_push_subs_user_id ON push_subscriptions(user_id);
CREATE INDEX idx_push_subs_deleted_at ON push_subscriptions(deleted_at);
