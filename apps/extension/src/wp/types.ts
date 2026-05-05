// WP REST API types for the extension poster module.

/** Credentials returned by /api/v1/wp-sites/by-domain/:domain (signed, extension-only). */
export type WPSiteCredentials = {
  base_url: string;
  app_username: string;
  app_password_plain: string;
};

/** Input for posting a backlink article via WP REST API. */
export type PostInput = {
  title: string;
  /** HTML content body containing the anchor link. */
  content_html: string;
};

/** Result from postBacklink — discriminated union on ok. */
export type PostResult =
  | {
      ok: true;
      postUrl: string;
      /** content.rendered trimmed to 16KB for evidence field. */
      evidence: string;
    }
  | {
      ok: false;
      errorCode: string;
      errorMessage?: string;
    };
