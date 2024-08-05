# diagram.py
from diagrams import Cluster, Diagram, Edge
from diagrams.onprem.compute import Server
from diagrams.onprem.database import Postgresql
from diagrams.onprem.vcs import Github
from diagrams.generic.device import Mobile, Tablet
from diagrams.onprem.client import Users
from diagrams.aws.iot import IotCertificate

spbase_attr = {
    "bgcolor": "lightgreen"
}

with Diagram("Flamingo Service Architecture",  filename="diagram", show=False):
    tablet = Tablet("Tablet users")
    mobile = Mobile("Mobile users")
    github = Github("Repository")
    with Cluster("Render"):
        frontend = Server("Frontend (React)")
        backend = Server("Backend (Go)")
        render = frontend - backend
    with Cluster("Superbase", graph_attr=spbase_attr):
        database = Postgresql("Database")
        auth = IotCertificate("Auth")
        sbase = [database, auth]
    
    backend >> database
    backend >> Edge(label="Authentication (Google)") >> auth
    frontend >> Edge(label="Authentication (Google)") >> auth
    github >> Edge(label="Deploy") >> render
    tablet >> frontend
    mobile >> frontend