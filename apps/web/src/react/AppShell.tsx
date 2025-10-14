import * as React from "react";
import { LanguageProvider } from "./LanguageProvider";
import Footer from "./Footer";
import { Toaster } from "@/components/ui/toaster";

export default function AppShell() {
  return (
    <LanguageProvider>
      <Toaster />
      <Footer />
    </LanguageProvider>
  );
}
