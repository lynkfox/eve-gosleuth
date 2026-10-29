"""Headless Pyfa bootstrap: stubs wx, points Pyfa at the eve.db built in pyfa_build, returns helpers."""
import os, sys
HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "stubs")); import sitecustomize_wx
sys.path.insert(0, os.path.join(HERE, "pyfa_build"))
os.chdir(os.path.join(HERE, "pyfa_build"))
import config
config.debug = False
config.defPaths(os.path.join(HERE, "save"))
import eos.db
import eos.events
eos.db.saveddata_meta.create_all(eos.db.saveddata_engine)
from eos.saveddata.character import Character
from eos.saveddata.fit import Fit
from eos.saveddata.damagePattern import DamagePattern
from eos.const import FittingSlot, ImplantLocation
from eos.saveddata.module import Module
from eos.const import FittingModuleState as State
import service.port.eft as eft
