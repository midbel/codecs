<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>

	<xsl:template match="/">
		<element>
			<xsl:apply-templates/>
		</element>
	</xsl:template>

	<xsl:template match="*">
		<element>
			<xsl:value-of select="fn:local-name()"/>
		</element>
	</xsl:template>

</xsl:stylesheet>