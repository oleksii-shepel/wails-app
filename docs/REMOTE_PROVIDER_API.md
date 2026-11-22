# Remote Provider API

Remote connections now resolve through one backend provider contract instead of a protocol switch in `RemoteConnect`.

Each provider declares:

- protocol and display name
- authentication model
- read/write policy
- hierarchy semantics
- connection construction
- mount identity, display name, and remote host identity

Current built-ins are SMB, S3, Google Drive, Dropbox, and OneDrive. `RemoteListProviders` exposes their descriptors to the frontend.

## OAuth and tokens

Google Drive, Dropbox, and OneDrive support OAuth with PKCE. OAuth sessions remain process-local: access and refresh tokens are not persisted. Access tokens are refreshed before expiry, and refresh exchange is serialized so concurrent VFS requests do not race the same refresh token.

## Cloud semantics

Google Drive and OneDrive are ID-backed API filesystems. S3 intentionally advertises `object-prefix` hierarchy rather than POSIX hierarchy: directories are synthesized from object-key prefixes, object reads are ranged, and rename/create/delete remain unavailable in the current read-only adapter.

Google Drive, Dropbox, and OneDrive serve read-only file handles through HTTP
byte ranges. Seeking reopens the content at the requested offset and sequential
reads advance in bounded ranges. If the provider sends a full response instead,
the adapter streams it and discards any prefix needed to reach the offset.

This provider contract is deliberately below the plugin layer. Phase 69 can make cloud providers ordinary plugin contributions without changing the connection/VFS semantics implemented here.
