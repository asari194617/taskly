import * as React from "react";
import { useLang } from "../LanguageProvider";
import TasksList from "../TasksList";

export default function HomePage() {
  const { dict } = useLang();
  const d = dict.home;

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <div className="space-y-2">
        <h1 className="text-2xl font-semibold">{d.title}</h1>
        <p className="text-sm text-muted-foreground">{d.description}</p>
      </div>
      <div>
        <TasksList />
      </div>
    </div>
  );
}
