# Goose Roadmap

## Actual features
* **User Authentication**: Integrated via GlueAuth with session management (`ensureUser`).
* **Profile Management**: View user profiles (`/user/{handle}`) including follower/following counts.
* **Social Graph**: Follow and Unfollow functionality (`POST/DELETE /follow`).
* **Honking**: Create short text posts ("honks") via `POST /publish`.
* **Replies**: Reply to existing honks via `POST /reply`.
* **Timelines**:
    *   **User Timeline**: View a specific user's posts.
    *   **Mainstream Feed**: Global or algorithmic feed of content (`/mainstream`).
    *   **Home Feed**: Personalized feed of followed users (implied by `renderHome`).
*   **Web Push Notifications**: Support for registering subscriptions (`/push/register`) using VAPID keys.
*   **InceptionDB Integration**: Data storage for users and tweets.
*   **SEO**: Sitemap generation (`/sitemap.xml`) for better indexing.
*   **Static Frontend**: Vue.js based frontend served via `statics/www`.

## Next features
1.  **Likes System**: Implement "Like" functionality with counters, unique user validation, and activity updates.
2.  **Re-honk**: Ability to share/re-post other users' honks to your own timeline.
3.  **Pagination**: Infinite scroll or page-based navigation for Feeds and Activity lists to support large datasets.
4.  **Edit & Delete Honks**: Allow users to modify or remove their own content.
5.  **Rich Text Support**: Auto-linkifying URLs, highlighting `@mentions`, and clickable `#hashtags`.
6.  **Media Embedding**: Embed players for resources like YouTube videos or image previews from links.
7.  **Search & Exploration**: Search functionality for users and hashtags; "Trending" topics.
8.  **Enhanced Notifications**: A dedicated UI to view interactions (new followers, likes, mentions).
9.  **Extended Profile Editing**: Interface to update nickname, avatar, and bio description directly.
10. **Activity Separation**: Distinct views for "Feed" (what you see) vs "Activity" (what you do/interactions).

## Will not do these features
*   **Native Video Hosting**: Hosting and streaming video files directly (will rely on embeds).
*   **Private Direct Messaging (DMs)**: Focus will remain on public broadcasting and replies.
*   **Algorithmic "Black Box" Feeds**: Feeds will remain chronological or based on transparent logic (like "Mainstream"), avoiding opaque manipulation.
*   **Paid/Premium Subscriptions**: No plans for "Blue check" monetization or paywalls.
*   **Livestreaming**: Real-time video broadcasting is out of scope.
