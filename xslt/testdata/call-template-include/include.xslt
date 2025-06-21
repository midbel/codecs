<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">

	<xsl:template name="foobar">
		<xsl:param name="build" select="'angle'"/>
		<build-with>
			<xsl:value-of select="$build"/>
		</build-with>
	</xsl:template>
	
</xsl:stylesheet>