

function PeerTimeline(id) {
  var self = this;

  let trace1 = {
    type: "scatter",
    mode: "lines",
    name: 'Observed peers',
    x: [],
    y: [],
    line: {color: '#17BECF'}
  }

  var layout = {
    xaxis: {
      autorange: true,
      rangeselector: {buttons: [
          {
            count: 1,
            label: '1w',
            step: 'week',
            stepmode: 'backward'
          },
          {
            count: 3,
            label: '3w',
            step: 'week',
            stepmode: 'backward'
          },
          {step: 'all'}
        ]},
      type: 'date'
    },
    yaxis: {
      autorange: true,
      type: 'linear'
    }
  };
  Plotly.newPlot(id, [trace1], layout);

  self.update = function(data) {
      trace1.x = [];
      trace1.y = [];
      for (const d of data) {
        trace1.x.push(d.Timestamp*1000);
        trace1.y.push(d.Value);
      }
      Plotly.react(id ,[trace1] , layout);
  };
}

function PeerGeoMap(id) {
	var self = this;

	let data = {
			type: 'scattergeo',
			mode: 'markers',
			lon: [],
			lat: [],
			marker: {
					size: 4,
					line: {
							width: 1
					}
			},
	};

	var layout = {
			geo: {
					scope: 'world',
					resolution: 50,
					showland: true,
					landcolor: '#EAEAAE',
					countrycolor: '#d3d3d3',
					countrywidth: 1.5,
					subunitcolor: '#d3d3d3'
			}
	};

	Plotly.newPlot(id, [data], layout); 
  self.update = function(pts) {
      data.lat = [];
      data.lon = [];
      for (const d of pts) {
        data.lat.push(d.Latitude);
        data.lon.push(d.Longitude);
      }
      Plotly.react(id, [data], layout);
  };
}
