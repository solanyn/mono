import json
from pathlib import Path

import pytest

FIXTURES_DIR = Path(__file__).parent / "fixtures"


@pytest.fixture
def rba_csv_text():
    return (FIXTURES_DIR / "rba_f1.csv").read_text()


@pytest.fixture
def abs_cpi_json():
    return json.loads((FIXTURES_DIR / "abs_cpi.json").read_text())


@pytest.fixture
def aemo_nem_json():
    return json.loads((FIXTURES_DIR / "aemo_nem.json").read_text())


@pytest.fixture
def reddit_hot_json():
    return json.loads((FIXTURES_DIR / "reddit_hot.json").read_text())


@pytest.fixture
def reddit_new_json():
    return json.loads((FIXTURES_DIR / "reddit_new.json").read_text())


@pytest.fixture
def rss_guardian_xml():
    return (FIXTURES_DIR / "rss_guardian.xml").read_text()


@pytest.fixture
def domain_auctions_json():
    return json.loads((FIXTURES_DIR / "domain_auctions.json").read_text())


@pytest.fixture
def domain_listings_json():
    return json.loads((FIXTURES_DIR / "domain_listings.json").read_text())
