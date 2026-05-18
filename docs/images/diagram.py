"""Architecture diagram for flamingo-armond.

Generates ``architecture.png`` in this directory: a single-image overview of
the production runtime topology and the flow of a typical authenticated
GraphQL request.

Run from this directory::

    python diagram.py

Layers shown:
  - Vercel hosts the Next.js 16 frontend (App Router).
  - Render hosts the Go / Echo v5 backend (gqlgen GraphQL endpoint).
  - Supabase hosts Postgres (with RLS) and Auth (JWT issuer + JWKS).
  - schema/*.graphql is the shared SDL feeding both codegen tools.
  - Cards content is periodically synced from a Notion page into Postgres.
"""

from diagrams import Cluster, Diagram, Edge
from diagrams.custom import Custom
from diagrams.gcp.security import Iam
from diagrams.onprem.ci import GithubActions
from diagrams.onprem.client import Users
from diagrams.onprem.database import PostgreSQL
from diagrams.onprem.tracing import Jaeger
from diagrams.programming.framework import GraphQL

# Vendor logos staged in icons/ — kept locally so the diagram is portable.
ICON_VERCEL = "icons/vercel.png"
ICON_RENDER = "icons/render.png"
ICON_SUPABASE = "icons/supabase.png"
ICON_NOTION = "icons/notion.png"

graph_attr = {
    "fontsize": "18",
    "splines": "spline",
    "pad": "0.5",
    "nodesep": "0.6",
    "ranksep": "1.2",
}

with Diagram(
    "flamingo-armond — Production architecture",
    filename="architecture",
    show=False,
    direction="LR",
    outformat="png",
    graph_attr=graph_attr,
):
    # External actors
    user = Users("End user\n(Browser)")
    google = Iam("Google OAuth")

    # Frontend on Vercel — single node, vendor logo (no cluster frame)
    web = Custom("Next.js 16\n(App Router: RSC + client)", ICON_VERCEL)

    # Shared GraphQL contract — sits on the wire between Vercel and Render and
    # is also the SDL source of truth driving both codegen tools.
    schema = GraphQL("GraphQL\n(schema/*.graphql)")

    # Backend on Render — single node, vendor logo (no cluster frame)
    backend = Custom(
        "Go / Echo v5 backend\n(gqlgen, GORM, JWT auth)",
        ICON_RENDER,
    )

    # Supabase — Auth (Supabase logo) + Postgres
    with Cluster("Supabase"):
        sb_auth = Custom(
            "Auth\nJWT issuer + JWKS\n+ OAuth broker",
            ICON_SUPABASE,
        )
        pg = PostgreSQL("Postgres\nRLS · public + auth schemas")

    # Notion — upstream content source for Cards, synced on a schedule.
    notion = Custom("Notion\n(Cards source page)", ICON_NOTION)

    # Optional telemetry sink
    otel = Jaeger("OTLP collector\n(optional)")

    # CI / scheduled jobs — single node
    ci = GithubActions("GitHub Actions\n(backend / frontend / e2e / readiness-ping)")

    # ────────────────────────────── Edges ──────────────────────────────

    # Sign-in / session flow
    user >> Edge(label="HTTPS") >> web
    web >> Edge(label="OAuth", style="dashed") >> google
    google >> Edge(label="redirect", style="dashed") >> sb_auth
    web >> Edge(label="getUser() / cookie rotation", style="dashed") >> sb_auth

    # GraphQL request path: Vercel → GraphQL contract → Render.
    # High weight + thick stroke pins the three onto the same horizontal rank
    # so GraphQL is visually sandwiched between the Vercel and Render clusters.
    web >> Edge(
        label="POST /query\n(APQ, Bearer JWT)",
        penwidth="2",
        weight="10",
    ) >> schema
    schema >> Edge(
        label="resolver dispatch",
        penwidth="2",
        weight="10",
    ) >> backend

    # Backend → Supabase
    backend >> Edge(label="JWKS fetch", style="dashed") >> sb_auth
    backend >> Edge(label="SQL (GORM)\n+ migrations on boot") >> pg

    # Telemetry
    backend >> Edge(label="OTLP / HTTP", style="dotted") >> otel

    # Same SDL feeds both sides via codegen.
    # constraint=false keeps these edges from influencing the rank assignment,
    # so the request-path spine above stays straight.
    schema >> Edge(
        label="graphql-codegen",
        style="dashed",
        color="darkgreen",
        constraint="false",
    ) >> web
    schema >> Edge(
        label="gqlgen",
        style="dashed",
        color="darkgreen",
        constraint="false",
    ) >> backend

    # CI / scheduled
    ci >> Edge(label="deploy hook /\n15-min /internal/ping", style="dotted") >> backend
    ci >> Edge(style="dotted") >> web

    # Scheduled Cards sync: Notion → backend → Postgres.
    notion >> Edge(
        label="scheduled sync\n(Cards content)",
        style="dashed",
        color="darkorange",
    ) >> backend
