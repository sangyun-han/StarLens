# Translations

StarLens UI languages live in this directory — **one JSON file per language**,
auto-discovered at build time. There is no registration file to edit.

## Adding a new language

1. Copy `en.json` (the source of truth) to `<code>.json`, where `<code>` is the
   language's BCP 47 code — `fr.json`, `ja.json`, `zh-Hans.json`, …
2. Set `meta.name` to the language's **native** name (e.g. `"Français"`,
   `"日本語"`). This is what the in-app language switcher displays.
3. Translate every value. Keep the keys and any `{{placeholders}}` exactly as
   they are — placeholders are substituted at runtime.
4. Run `npm run build` to confirm the JSON is valid. Done — the language appears
   in the switcher automatically.

## Rules of thumb

- `en.json` is the reference file: new UI strings land there first, and other
  languages fall back to English for any key they have not translated yet.
  A partial translation is fine to submit.
- Plural forms use i18next suffixes (`_one`, `_other`, …) driven by
  `Intl.PluralRules`. Languages without a singular/plural distinction (Korean,
  Japanese, Chinese, …) only need `_other`.
- Keys under `loads.empty` (and any string rendered with `<Trans>`) may contain
  tags like `<code>…</code>` — keep the tags, translate the text around them.
- Product names (StarRocks, Kafka, `SHOW FRONTENDS`, env var names) and the
  `runMode` technical labels are usually best left untranslated.

Backend-generated text (alert messages delivered to webhooks and logs) is
intentionally English-only so downstream receivers see a consistent format.
