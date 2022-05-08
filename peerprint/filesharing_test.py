import unittest
import tempfile
from pathlib import Path
from filesharing import pack_job, unpack_job 

class TestPackJob(unittest.TestCase):
    def setUp(self):
        self.tempdir = tempfile.TemporaryDirectory()
        p = Path(self.tempdir.name)
        self.paths = [
            (p / 'a.gcode'),
            (p / 'b.gcode'),
            (p / 'c.gcode'),
        ]
        for path in self.paths:
            path.touch()

    def tearDown(self):
        self.tempdir.cleanup()

    def test_pack_job_with_files(self):
        manifest = dict(man='ifest')
        outpath = tempfile.NamedTemporaryFile(suffix=".zip")
        with pack_job(manifest, [str(p) for p in self.paths], outpath.name) as f:
            with tempfile.TemporaryDirectory() as td:
                result = unpack_job(outpath, td)
                self.assertEqual(result[0], manifest)
                self.assertEqual([Path(p).name for p in result[1]], [p.name for p in self.paths])

    def test_pack_job_hash_matching(self):
        manifest = dict(man='ifest')
        tf1 = tempfile.NamedTemporaryFile(suffix=".zip")
        tf2 = tempfile.NamedTemporaryFile(suffix=".zip")
        with pack_job(manifest, [str(p) for p in self.paths], tf1.name) as f1:
            with pack_job(manifest, [str(p) for p in self.paths], tf2.name) as f2:
                self.assertEqual(f1.job_hash, f2.job_hash)

    def test_pack_job_hash_not_matching(self):
        manifest = dict(man='ifest')
        tf1 = tempfile.NamedTemporaryFile(suffix=".zip")
        tf2 = tempfile.NamedTemporaryFile(suffix=".zip")
        with pack_job(manifest, [str(p) for p in self.paths], tf1.name) as f1:
            with pack_job(manifest, [str(p) for p in self.paths[1:]], tf2.name) as f2:
                self.assertNotEqual(f1.job_hash, f2.job_hash)
        

    def test_manifest_references_all_files(self):
        #raise NotImplementedError
        pass
