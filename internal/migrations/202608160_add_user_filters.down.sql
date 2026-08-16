DROP INDEX idx_filter_keywords_filter_id ON filter_keywords;
DROP INDEX idx_user_filters_user_id ON user_filters;

DROP TABLE IF EXISTS filter_keywords;
DROP TABLE IF EXISTS user_filters;
