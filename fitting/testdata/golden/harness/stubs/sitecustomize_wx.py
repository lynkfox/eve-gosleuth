# Meta-path finder: any "wx" / "wx.*" import returns a MagicMock module.
import sys, importlib.abc, importlib.machinery, types
from unittest.mock import MagicMock
class _Loader(importlib.abc.Loader):
    def create_module(self, spec):
        m = MagicMock(name=spec.name); m.NewEvent.side_effect = lambda *a, **k: (MagicMock(), MagicMock())
        m.__path__ = []
        m.__name__ = spec.name
        m.__spec__ = spec
        m.__loader__ = self
        return m
    def exec_module(self, module): pass
class _Finder(importlib.abc.MetaPathFinder):
    def find_spec(self, name, path, target=None):
        if name in ("wx", "gui", "graphs") or name.split(".")[0] in ("wx", "gui", "graphs"):
            return importlib.machinery.ModuleSpec(name, _Loader(), is_package=True)
        return None
sys.meta_path.append(_Finder())
