-- migration: 000007_create_webhook_subscriptions.down.sql
-- drops the webhook_subscriptions table

DROP TABLE IF EXISTS pulse.webhook_subscriptions;
