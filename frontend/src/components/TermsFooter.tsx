import { Link } from "react-router-dom";
import { useI18n } from "../lang/I18nProvider";

export default function TermsFooter() {
  const { t } = useI18n();
  return (
    <footer className="mt-auto bg-slate-900/80 py-4 text-center text-xs text-slate-400">
      <div className="container mx-auto flex flex-col items-center justify-center gap-1 px-4 sm:flex-row">
        <span>{t("termsFooter.disclaimer")}</span>
        <Link to="/terms" className="font-semibold text-blue-300 transition hover:text-blue-200">
          {t("termsFooter.cta")}
        </Link>
      </div>
    </footer>
  );
}
