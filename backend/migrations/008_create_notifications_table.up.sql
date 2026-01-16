-- Create notifications table
CREATE TABLE notifications (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id),
  type VARCHAR(50) NOT NULL,
  related_post_id UUID REFERENCES posts(id),
  related_comment_id UUID REFERENCES comments(id),
  related_user_id UUID REFERENCES users(id),
  read_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT now()
);

-- Create indexes for notifications table
CREATE INDEX idx_notifications_user_id ON notifications(user_id);
CREATE INDEX idx_notifications_user_read ON notifications(user_id, read_at);
CREATE INDEX idx_notifications_created_at ON notifications(created_at DESC);
CREATE INDEX idx_notifications_type ON notifications(type);
