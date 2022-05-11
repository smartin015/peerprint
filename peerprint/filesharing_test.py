import unittest
import tempfile
from pathlib import Path
from filesharing import pack_job, unpack_job 

class TestPackJob(unittest.TestCase):
    def setUp(self):
        self.tempdir = tempfile.TemporaryDirectory()
        p = Path(self.tempdir.name)
        self.paths = dict([(n, p / n) for n in ['a.gcode', 'b.gcode', 'c.gcode']])
        for path in self.paths.values():
            path.touch()

    def tearDown(self):
        self.tempdir.cleanup()

    def test_pack_job_with_files(self):
        manifest = dict(man='ifest')
        with tempfile.NamedTemporaryFile(suffix=".zip") as outpath:
            pack_job(manifest, self.paths, outpath.name)
            with tempfile.TemporaryDirectory() as td:
                result = unpack_job(outpath.name, td)
                self.assertEqual(result[0], manifest)
                self.assertEqual([Path(p).name for p in result[1]], list(self.paths.keys()))

    def test_pack_job_hash_matching(self):
        manifest = dict(man='ifest')
        with tempfile.NamedTemporaryFile(suffix=".zip") as tf1:
            with tempfile.NamedTemporaryFile(suffix=".zip") as tf2:
                h1 = pack_job(manifest, self.paths, tf1.name)
                h2 = pack_job(manifest, self.paths, tf2.name)
                self.assertEqual(h1, h2)

    def test_pack_job_hash_not_matching(self):
        manifest = dict(man='ifest')
        with tempfile.NamedTemporaryFile(suffix=".zip") as tf1:
            with tempfile.NamedTemporaryFile(suffix=".zip") as tf2:
                h1 = pack_job(manifest, self.paths, tf1.name)
                h2 = pack_job(manifest, dict(list(self.paths.items())[1:]), tf2.name)
                self.assertNotEqual(h1, h2)
        

    def test_manifest_references_all_files(self):
        #raise NotImplementedError
        pass
