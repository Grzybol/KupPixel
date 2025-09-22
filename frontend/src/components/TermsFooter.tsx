import { Link } from "react-router-dom";

export default function TermsFooter() {
  return (
    <footer className="mt-auto bg-slate-900/80 py-4 text-center text-xs text-slate-400">
      <div className="container mx-auto flex flex-col items-center justify-center gap-1 px-4 sm:flex-row">
        <span>Korzystanie z serwisu oznacza akceptację regulaminu.</span>
        <Link
          to="/terms"
          className="font-semibold text-blue-300 transition hover:text-blue-200"
        >
          Przeczytaj regulamin
        </Link>
      </div>
    </footer>
  );
}
