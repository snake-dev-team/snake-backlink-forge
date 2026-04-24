-- +goose Up
-- 30 dork patterns: 8 blog_comment + 8 forum_profile + 8 web2_post + 6 directory_listing
-- Language mix: ~6 VN patterns (20%), ~24 EN patterns (80%)
-- {niche} is a Go template placeholder replaced at query-build time via parameterized builder (NOT SQL interpolation)

INSERT INTO dork_patterns (pattern, target_type, expected_platform, success_weight) VALUES

-- ── blog_comment (8) ─────────────────────────────────────────────────────────
('site:wordpress.com "{niche}" "leave a reply"',                          'blog_comment', 'wordpress',   0.600),
('"{niche}" "powered by wordpress" inurl:/comments/',                     'blog_comment', 'wordpress',   0.550),
('"{niche}" "leave a comment" -inurl:signin -inurl:login',                'blog_comment', 'wordpress',   0.520),
('site:*.blogspot.com "{niche}" inurl:/comments/',                        'blog_comment', 'blogger',     0.500),
('"{niche}" "comments powered by disqus"',                                'blog_comment', 'disqus',      0.480),
('"{niche}" "post a comment" inurl:blog',                                 'blog_comment', 'generic',     0.450),
('"{niche}" inurl:/blog/ "leave a reply"',                                'blog_comment', 'wordpress',   0.470),
('"bài viết về {niche}" "để lại bình luận"',                              'blog_comment', 'wordpress',   0.420),

-- ── forum_profile (8) ────────────────────────────────────────────────────────
('"{niche}" "powered by vbulletin" inurl:/members/',                      'forum_profile', 'vbulletin',  0.520),
('"{niche}" "powered by phpbb" inurl:memberlist.php',                     'forum_profile', 'phpbb',      0.500),
('"{niche}" "discourse" inurl:/u/',                                       'forum_profile', 'discourse',  0.560),
('"{niche}" "powered by flarum" inurl:/u/',                               'forum_profile', 'flarum',     0.480),
('"{niche}" "powered by mybb" inurl:member.php',                          'forum_profile', 'mybb',       0.450),
('"{niche}" "register" inurl:forum inurl:/register',                      'forum_profile', 'generic',    0.400),
('"{niche}" "new user" inurl:forum "create account"',                     'forum_profile', 'generic',    0.380),
('"diễn đàn {niche}" "đăng ký thành viên"',                               'forum_profile', 'generic',    0.370),

-- ── web2_post (8) ────────────────────────────────────────────────────────────
('site:medium.com "{niche}" -inurl:signin',                               'web2_post', 'medium',         0.650),
('site:*.blogspot.com "{niche}"',                                         'web2_post', 'blogger',        0.580),
('site:*.wordpress.com "{niche}" -inurl:login',                           'web2_post', 'wordpress_com',  0.600),
('site:*.tumblr.com "{niche}" -inurl:login',                              'web2_post', 'tumblr',         0.500),
('site:ghost.io "{niche}" -inurl:signin',                                 'web2_post', 'ghost',          0.480),
('site:dev.to "{niche}" -inurl:signin',                                   'web2_post', 'devto',          0.620),
('site:hashnode.com "{niche}" -inurl:signin',                             'web2_post', 'hashnode',       0.560),
('site:medium.com "{niche}" "tiếng việt"',                                'web2_post', 'medium',         0.500),

-- ── directory_listing (6) ────────────────────────────────────────────────────
('"add your {niche} business" "directory" "submit"',                      'directory_listing', 'generic', 0.420),
('"{niche}" "business directory" "add listing"',                          'directory_listing', 'generic', 0.440),
('"submit {niche} website" inurl:/submit',                                'directory_listing', 'generic', 0.400),
('"{niche} directory" "free submission"',                                 'directory_listing', 'generic', 0.420),
('inurl:/add-url "{niche}"',                                              'directory_listing', 'generic', 0.380),
('"thư mục {niche}" "đăng ký miễn phí"',                                  'directory_listing', 'generic', 0.360);
