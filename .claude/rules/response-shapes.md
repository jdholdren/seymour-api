---
paths:
  - "internal/api/**/*.go"
---

# Response shape checks

Handler functions in this package don't declare a return type — the response
shape is whatever gets passed to `writeJSON(w, status, ...)` inside the
handler body. Because of that, the compiler won't catch a breaking change to
a response shape the way it would for a typed return value.

Whenever you touch a handler in this package (or a helper it calls to build
its response, e.g. `apiSubscription`, `apiFilter`), read the full body of
that handler down to its `writeJSON` call(s) and check whether the shape
being written has changed in a breaking way for existing clients:
- a field removed or renamed
- a field's type changed (e.g. `string` -> `*string`, flattened -> nested)
- the top-level struct swapped for a different one (e.g. returning
  `SubscriptionResp` where `FeedResp` used to go out)
- a previously-required field now omitted/empty in some path

If you find a breaking change, call it out explicitly to the user rather than
assuming it's fine — even if the new shape is arguably better. Additive
changes (new optional fields) aren't breaking and don't need a callout.
