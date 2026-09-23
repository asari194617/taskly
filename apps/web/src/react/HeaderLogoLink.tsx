import * as React from "react";
import { gql } from "@/lib/graphql";

export default function HeaderLogoLink({
  children,
}: {
  children: React.ReactNode;
}) {
  const [href, setHref] = React.useState("/");

  React.useEffect(() => {
    let active = true;
    gql<{ me: { id: number } }>("query{ me{ id } }", undefined, {
      redirectOnUnauthorized: false,
    })
      .then((data) => {
        if (active && data.me?.id) setHref("/home");
      })
      .catch(() => {
        // Keep the public top page as a usable fallback for signed-out users
        // and while the authentication service is unavailable.
      });
    return () => {
      active = false;
    };
  }, []);

  return (
    <a href={href} className="font-semibold" aria-label="Taskly">
      {children}
    </a>
  );
}
