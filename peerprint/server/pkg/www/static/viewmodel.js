
function AppViewModel() {
  let self = this;
  self.history = ko.observableArray([]);
	self.serverSummary = ko.observable();
	self.storageSummary = ko.observable();
  self.events = ko.observableArray([]);

	self.updateHistory = function() {
		$.getJSON("/history", function(data) { 
			self.history(data);
		});
	}
	self.updateServerSummary = function() {
		$.getJSON("/serverSummary", function(data) { 
			self.serverSummary(data);
		});
	};
	self.updateStorageSummary = function() {
		$.getJSON("/storageSummary", function(data) { 
			self.storageSummary(data);
		});
	};
	self.updateEvents = function() {
		$.get("/events", function(data) { 
      let result = [];
      for (let line of data.split('\n')) {
        if (line.trim() === "") {
          continue;
        }
        console.log("parse", line);
        result.push(JSON.parse(line));
      }
			self.events(result);
		});
	}
	self.updateHistory();
	self.updateServerSummary();
	self.updateStorageSummary();
  self.updateEvents();
}
