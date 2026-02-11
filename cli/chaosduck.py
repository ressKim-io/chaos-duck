#!/usr/bin/env python3
"""ChaosDuck CLI - Chaos Engineering for K8s & AWS."""

import json
import sys

import click
import urllib.request
import urllib.error

BASE_URL = "http://localhost:8000"


def _api_get(path: str) -> dict:
    """Make a GET request to the API."""
    try:
        req = urllib.request.Request(f"{BASE_URL}{path}")
        with urllib.request.urlopen(req) as resp:
            return json.loads(resp.read())
    except urllib.error.URLError as e:
        click.echo(f"Error: Cannot connect to backend at {BASE_URL} - {e}", err=True)
        sys.exit(1)


def _api_post(path: str, data: dict = None) -> dict:
    """Make a POST request to the API."""
    try:
        body = json.dumps(data or {}).encode()
        req = urllib.request.Request(
            f"{BASE_URL}{path}",
            data=body,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        with urllib.request.urlopen(req) as resp:
            return json.loads(resp.read())
    except urllib.error.URLError as e:
        click.echo(f"Error: Cannot connect to backend at {BASE_URL} - {e}", err=True)
        sys.exit(1)


@click.group()
@click.option("--url", default=BASE_URL, help="Backend API URL")
def cli(url):
    """ChaosDuck - Chaos Engineering CLI for K8s & AWS."""
    global BASE_URL
    BASE_URL = url


@cli.command()
@click.argument("name")
@click.option("--type", "chaos_type", required=True, help="Chaos type (e.g. pod_delete, network_latency)")
@click.option("--namespace", default="default", help="Target namespace")
@click.option("--labels", default=None, help="Target labels (key=value,key=value)")
@click.option("--timeout", default=30, help="Timeout in seconds")
@click.option("--dry-run", is_flag=True, help="Dry run mode")
@click.option("--param", multiple=True, help="Extra parameters (key=value)")
def run(name, chaos_type, namespace, labels, timeout, dry_run, param):
    """Run a chaos experiment."""
    target_labels = {}
    if labels:
        for pair in labels.split(","):
            k, v = pair.split("=", 1)
            target_labels[k] = v

    parameters = {}
    for p in param:
        k, v = p.split("=", 1)
        parameters[k] = v

    config = {
        "name": name,
        "chaos_type": chaos_type,
        "target_namespace": namespace,
        "target_labels": target_labels or None,
        "parameters": parameters,
        "safety": {
            "timeout_seconds": timeout,
            "dry_run": dry_run,
        },
    }

    endpoint = "/api/chaos/dry-run" if dry_run else "/api/chaos/experiments"
    result = _api_post(endpoint, config)
    click.echo(json.dumps(result, indent=2))


@cli.command()
@click.option("--id", "experiment_id", default=None, help="Experiment ID")
def status(experiment_id):
    """Check experiment status."""
    if experiment_id:
        result = _api_get(f"/api/chaos/experiments/{experiment_id}")
    else:
        result = _api_get("/api/chaos/experiments")
    click.echo(json.dumps(result, indent=2))


@cli.command()
@click.argument("experiment_id")
def rollback(experiment_id):
    """Rollback an experiment."""
    result = _api_post(f"/api/chaos/experiments/{experiment_id}/rollback")
    click.echo(json.dumps(result, indent=2))


@cli.command()
def stop():
    """Trigger emergency stop."""
    if click.confirm("Trigger Emergency Stop? This will rollback ALL active experiments."):
        result = _api_post("/emergency-stop")
        click.echo(json.dumps(result, indent=2))


@cli.command()
@click.option("--namespace", default="default", help="K8s namespace")
@click.option("--provider", type=click.Choice(["k8s", "aws", "combined"]), default="combined")
def topology(namespace, provider):
    """View infrastructure topology."""
    if provider == "k8s":
        result = _api_get(f"/api/topology/k8s?namespace={namespace}")
    elif provider == "aws":
        result = _api_get("/api/topology/aws")
    else:
        result = _api_get(f"/api/topology/combined?namespace={namespace}")
    click.echo(json.dumps(result, indent=2))


@cli.command()
@click.argument("experiment_id")
def analyze(experiment_id):
    """Analyze an experiment with AI."""
    result = _api_post(f"/api/analysis/experiment/{experiment_id}")
    click.echo(json.dumps(result, indent=2))


@cli.command()
def health():
    """Check backend health."""
    result = _api_get("/health")
    click.echo(json.dumps(result, indent=2))


if __name__ == "__main__":
    cli()
