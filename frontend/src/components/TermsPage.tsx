import { useI18n } from "../lang/I18nProvider";

export default function TermsPage() {
  const { t, dictionary } = useI18n();
  const terms = (dictionary.terms as Record<string, unknown>) ?? {};
  const sections = Array.isArray(terms.sections)
    ? (terms.sections as Array<{ title?: unknown; paragraphs?: unknown; list?: unknown }>)
    : [];

  return (
    <div className="bg-slate-950 text-slate-100">
      <div className="mx-auto max-w-4xl px-6 py-12">
        <header className="mb-10">
          <h1 className="text-4xl font-bold text-blue-300">{t("terms.title")}</h1>
          <p className="mt-4 text-sm text-slate-300">{t("terms.intro")}</p>
        </header>

        <section className="space-y-4">
          {sections.map((section, index) => {
            const title = typeof section.title === "string" ? section.title : undefined;
            const paragraphs = Array.isArray(section.paragraphs)
              ? (section.paragraphs as string[])
              : [];
            const listItems = Array.isArray(section.list) ? (section.list as string[]) : [];
            return (
              <div key={title ?? `section-${index}`}>
                {title && <h2 className="text-2xl font-semibold text-blue-200">{title}</h2>}
                {paragraphs.map((paragraph, paragraphIndex) => (
                  <p
                    key={`paragraph-${paragraphIndex}`}
                    className={`text-sm leading-relaxed text-slate-200${paragraphIndex === 0 ? " mt-2" : ""}`}
                  >
                    {paragraph}
                  </p>
                ))}
                {listItems.length > 0 && (
                  <ul className="ml-6 list-disc space-y-1 text-sm text-slate-200">
                    {listItems.map((item, listIndex) => (
                      <li key={`list-${listIndex}`}>{item}</li>
                    ))}
                  </ul>
                )}
              </div>
            );
          })}
        </section>
      </div>
    </div>
  );
}
