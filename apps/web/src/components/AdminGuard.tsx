"use client";

import React, { useEffect, useState } from "react";
import { gql } from "../lib/graphql";

type MeResult = {
  me?: {
    id: number;
    email: string;
    is_admin: boolean;
    is_manager: boolean;
  } | null;
};

export default function AdminGuard() {
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function checkAdmin() {
      try {
        const data = await gql<MeResult>(
          "query{ me{ id email is_admin is_manager } }"
        );
        if (cancelled) return;
        const me = data?.me;
        if (!(me && (me.is_admin || me.is_manager))) {
          window.location.href = "/403";
          return;
        }
      } catch (e: any) {
        if (cancelled) return;
        const message = e?.message || String(e);
        setError(message);
      }
    }

    checkAdmin();

    return () => {
      cancelled = true;
    };
  }, []);

  if (error) {
    return (
      <div
        id="admin-guard-error"
        style={{
          padding: "12px",
          margin: "12px 0",
          border: "1px solid #e53e3e",
          background: "#fff5f5",
          color: "#c53030",
          borderRadius: "6px",
          fontSize: "14px",
        }}
      >
        {error}
      </div>
    );
  }

  return null;
}
