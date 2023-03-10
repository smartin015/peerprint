
function AppViewModel(hash) {
  let self = this;
  self.serverSummary = ko.observable();
  self.storageSummary = ko.observable();
  self.instances = ko.observableArray([]);
  self.credentials = ko.observableArray([]);
  self.connections = ko.observableArray([]);
  self.events = ko.observableArray([]);
  self.peerLogs = ko.observableArray([]);
  self.lobby = ko.observableArray([]);
  self.lobbyStats = ko.observable({});
  self.runtil = ko.observable("n/a");
  self.registry = ko.observableArray([]);
	self.toastData = ko.observable({title: "", body: ""});
	self.showToast = function(title, body) {
		self.toastData({title, body});
	};
  self.timeline = new PeerTimeline('peerTimeline');
  self.geo = new PeerGeoMap('peerGeo');

  setInterval(() => {
    const ls = self.lobbyStats();
    if (ls && ls.local && ls.local.Deadline !== undefined) {
      const ts = new Date(ls.local.Deadline);
      const dt = ts - (new Date());
      if (dt > 0) {
        self.runtil(`${Math.floor(dt/1000)}s`);
      } else {
        self.runtil("n/a");
      }
    }
  }, 500);

	self.shownTab = ko.observable(null);
	self.gotoTab = function(v, e) {
		const tabel = document.querySelector('#' + v + '-tab');
		if (tabel) {
			const tab = new bootstrap.Tab(tabel);
			tab.show();
			self.shownTab(v);
		}
    if (e !== undefined) {
      e.preventDefault();
    }
	}
	self.gotoTab(hash[1] || 'server');
  self.selectedInstance = ko.observable(hash[0] !== 'null' ? hash[0] : null);
  self.setInstance = function(i) {
    self.selectedInstance(i);
  }

  self.genPSK = function() {
    throw new Error("todo");
  }

  $("#myTab a").on("shown.bs.tab", function(e) {
		let tid = $(e.target).attr("id");
    self.shownTab(tid.split('-tab')[0]);
    self.refresh();
  });

	let toast = new bootstrap.Toast($("#toast"));
	self.toastData.subscribe(function(v) {
		toast.show();
	});

  self._streamingGet = function(url, req, obs) {
    $.get(url, req, function(data) { 
      let result = [];
      for (let line of data.split('\n')) {
        if (line.trim() === "") {
          continue;
        }
        result.push(JSON.parse(line));
      }
      obs(result);
    });
  }

  self.updatePeerLogs = function() {
    self._streamingGet("/peerLogs", {instance: self.selectedInstance()}, self.peerLogs);
  };

  self.updateTimeline = function() {
    self._streamingGet("/timeline", {instance: self.selectedInstance()}, self.timeline.update);
  };

  self.updateGeo = function() {
    self._streamingGet("/clients/location", {instance: self.selectedInstance()}, self.geo.update);
  };

  self.setStatus = function() {
    var data = $('#setstatus input').toArray().reduce(function(obj, item) {
          obj[item.id] = item.value;
          return obj;
    }, {});
    $("#newconn input").each(function(i,v){v.value = ""});
    $.post("/clients/set_status", data).done((data) => {
			self.showToast("Success", "Connection created");
			self.gotoTab("conns")
    }).fail(() => {
			self.showToast("Error", "Failed to create connection");
    });
  }

  self.updateServerSummary = function() {
    $.getJSON("/serverSummary", {instance: self.selectedInstance()}, function(data) { 
      self.serverSummary(data);
    });
  };
  self.updateStorageSummary = function() {
    $.getJSON("/storageSummary", {instance: self.selectedInstance()}, function(data) { 
      self.storageSummary(data);
    });
  };
  self.updateEvents = function() {
    self._streamingGet("/events", {instance: self.selectedInstance()}, self.events);
  };
  self.syncManual = function() {
    $.getJSON("/server/sync", {}, function() {
			self.showToast("Success", "Synced with other peers");
      self.updateStorageSummary();
    });
  }
  self.syncLobby = function() {
    $.getJSON("/lobby/sync", {seconds: 60}, function() {
      console.log("lobby sync'd");
      self.updateLobby();
    });
  }
  self.updateLobby = function() {
    self._streamingGet("/lobby", undefined, (data) => {
      self.lobbyStats({local: data[0], world: data[1]});
      self.lobby(data.slice(2));
    });
  };
  self.updateRegistry = function() {
    self._streamingGet("/registry", undefined, self.registry);
  };
  
  
  self.updateInstances = function() {
    self._streamingGet("/connection", undefined, (data) => {
			let names = [];
			for (let c of data) {
				names.push(c.network);
			}
      self.instances(names);
			self.connections(data);
    });
  };

  self.refreshTS = ko.observable(new Date());
  self.refresh = function() {
    self.updateInstances();
    if (!self.selectedInstance()) {
      return;
    }
    switch (self.shownTab()) {
      case "timeline":
        self.updateTimeline();  
        self.updatePeerLogs();
        break;
      case "server":
        self.updateServerSummary();
        break;
      case "storage":
        self.updateStorageSummary();
        break;
      case "events":
        self.updateEvents();
        break;
      case "lobby":
        self.updateLobby(true);
        break;
      case "geography":
        self.updateGeo();
      case "registry":
        self.updateRegistry();
    }
    self.refreshTS(new Date());
  };
  self.refresh();
  self.loop = setInterval(self.refresh, 5*1000);

  self.newConnection = function() {
    var data = $('#newconn input').toArray().reduce(function(obj, item) {
          obj[item.id] = item.value;
          return obj;
    }, {});
    $("#newconn input").each(function(i,v){v.value = ""});
    $.post("/connection/new", data).done((data) => {
			self.showToast("Success", "Connection created");
			self.gotoTab("conns")
    }).fail(() => {
			self.showToast("Error", "Failed to create connection");
    });
  };

  self.newRegistry = function(v) {
    var data = $('#newregistry input').toArray().reduce(function(obj, item) {
          obj[item.id] = item.value;
          return obj;
    }, {});
    $("#newregistry input").each(function(i,v){v.value = ""});
    $.post("/registry/new", data).done((data) => {
			self.showToast("Success", "Network published successfully");
      self.gotoTab("registry");
    }).fail(() => {
			self.showToast("Error", "Failed to publish network");
    });
  };

  self.deleteRegistry = function(v) {
    $.post("/registry/delete", {uuid: v.uuid, local: v.local}).done((data) => {
			self.showToast("Success", "Network published successfully");
      self.gotoTab("registry");
    }).fail(() => {
			self.showToast("Error", "Failed to publish network");
    });
  }

  self.newPassword = function(v) {
    var data = $('#newpassword input').toArray().reduce(function(obj, item) {
          obj[item.id] = item.value;
          return obj;
    }, {});
    $.post("/password/new", data).done((data) => {
			self.showToast("Success", "Password changed successfully");
    }).fail(() => {
			self.showToast("Error", "Failed to change password");
    });
  };

	self.gotoPublish = function(v, e) {
		let advform = $("#newregistry");
		advform.find("#name").val(v.network);
		advform.find("#rendezvous").val(v.rendezvous);
		advform.find("#creator").val(v.display_name);
		advform.find("#local").val(v.local);
		self.gotoTab("newregistry", e);
	};
  self.gotoConnect = function(v, e) {
		let advform = $("#newconn");
		advform.find("#name").val(v.network);
		advform.find("#rendezvous").val(v.rendezvous);
		advform.find("#local").val(v.local);
		self.gotoTab("newconn", e);
  };

	self.deleteConn = function(v, e) {
		console.log("deleting", v, e)
    $.post("/connection/delete", {network: v.network}).done((data) => {
			self.showToast("Success", "Deleted connection: " + v.network);
    }).fail(() => {
			self.showToast("Error", "Failed to delete connection");
    });
		e.preventDefault();
	};

  self.href = ko.computed(function() {
    let inst = self.selectedInstance();
    let tab = self.shownTab();
    if (!inst || !tab) {
      return;
    }
    window.location.hash =  `${inst}.${tab}`;
    console.log("set href");
    self.refresh();
  });


  self.getWebAuthnCredentials = function() {
    $.getJSON("/register/credentials", 
        {instance: self.selectedInstance()}, self.credentials);
  };
  self.getWebAuthnCredentials();

  self.removeWebAuthnCredential = function(id) {
    $.post('/register/remove', {id})
      .then((result) => {
        self.showToast("Success", "Removed credential");
        self.getWebAuthnCredentials();
      })
      .catch((err) => {
        self.showToast("Error", "Failed to remove (see console)");
        console.error(err);
      });
  }

	// ArrayBuffer to URLBase64
	function bufferEncode(value) {
		return btoa(String.fromCharCode.apply(null, new Uint8Array(value)))
			.replace(/\+/g, "-")
			.replace(/\//g, "_")
			.replace(/=/g, "");;
	}
  // Base64 to ArrayBuffer
  function bufferDecode(base64url) {
    base64url = base64url.toString();
    base64 =  base64url.replace(/\-/g, "+").replace(/_/g, "/");
    return Uint8Array.from(atob(base64), c => c.charCodeAt(0));
  }

  self.registerUser = function() {
    $.get(
      '/register/begin', null, function (data) { return data }, 'json')
      .then((ccOpts) => {
        console.log(ccOpts);
        ccOpts.publicKey.challenge = bufferDecode(ccOpts.publicKey.challenge);
        ccOpts.publicKey.user.id = bufferDecode(ccOpts.publicKey.user.id);
        return navigator.credentials.create({
          publicKey: ccOpts.publicKey
        })
      })
      .then((credential) => {
        console.log(credential);
        return $.post(
          '/register/finish',
          JSON.stringify({
            id: credential.id,
            rawId: bufferEncode(credential.rawId),
            type: credential.type,
            response: {
              attestationObject: bufferEncode(credential.response.attestationObject),
              clientDataJSON: bufferEncode(credential.response.clientDataJSON),
            },
          }),
          function (data) { return data },
          'json')
      })
      .then((success) => {
			  self.showToast("Success", "Device enrolled successfully");
        self.getWebAuthnCredentials();
      })
      .catch((error) => {
        console.log(error);
			  self.showToast("Error", "Failed enrollment (see console)");
      })
  };
}

